package social

import (
	"encoding/hex"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/data"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	consensus *consensus.Layer
	gossip    *gossip.Layer
	data      *data.Layer

	Config    *peer.Configuration
	FeedStore *feed.Store
	UserID    string
}

func Construct(config *peer.Configuration,
	data *data.Layer,
	consensus *consensus.Layer,
	gossip *gossip.Layer,
	hashedPublicKey [32]byte) *Layer {
	// Convert the byte array into a hex string.
	userID := hex.EncodeToString(hashedPublicKey[:])
	return &Layer{
		consensus: consensus,
		data:      data,
		gossip:    gossip,
		Config:    config,
		FeedStore: feed.LoadStore(),
		UserID:    userID,
	}
}

func (l *Layer) GetAddress() string {
	return l.consensus.GetAddress()
}

func (l *Layer) GetUserID() string {
	return l.UserID
}

func (l *Layer) Register() error {
	utils.PrintDebug("social", l.GetAddress(), "has initiated registration with id", l.UserID)
	newUserMsg := NewUserMessage{UserID: l.UserID}
	trspMsg, _ := l.Config.MessageRegistry.MarshalMessage(&newUserMsg)
	return l.gossip.Broadcast(trspMsg)
}

func (l *Layer) ProposeNewPost(info feed.PostInfo) error {
	utils.PrintDebug("social", l.UserID, "is proposing a new post")
	val := feed.MakeCustomPaxosValue(info)
	paxosVal := types.PaxosValue{
		UniqID:      xid.New().String(),
		CustomValue: val,
	}
	protocolID := feed.FeedIDFromUserID(l.UserID)
	err := l.consensus.ProposeWithProtocol(protocolID, paxosVal)
	if err != nil {
		return err
	}
	return nil
}

// FeedBlockchainUpdater takes a user id and returns a paxos feed blockchain updater.
func FeedBlockchainUpdater(feedStore *feed.Store, userID string) paxos.BlockchainUpdater {
	return func(config *peer.Configuration, newBlock types.BlockchainBlock) {
		utils.PrintDebug("social", "Updating local feed...")
		// Update the feed, also appending to the appropriate blockchain.
		feedStore.UpdateFeed(config.BlockchainStorage, userID, newBlock)
	}
}

// FeedProposalChecker takes a user id and returns a paxos proposal checker.
func FeedProposalChecker(userID string) paxos.ProposalChecker {
	return func(configuration *peer.Configuration, message types.PaxosProposeMessage) bool {
		// TODO Check remaining credits, timestamp etc.
		utils.PrintDebug("social", "Checking post proposal...")
		_ = feed.ParseCustomPaxosValue(message.Value.CustomValue)
		return true
	}
}

// FeedBlockGenerator takes a user id and returns a paxos feed block generator.
func FeedBlockGenerator(userID string) paxos.BlockGenerator {
	return func(config *peer.Configuration, msg types.PaxosAcceptMessage) types.BlockchainBlock {
		utils.PrintDebug("social", "Generating feed block...")
		prevHash := make([]byte, 32)
		// Get the blockchain store associated with the user's feed.
		blockchainStore := config.BlockchainStorage.GetStore(feed.FeedIDFromUserID(userID))
		// Get the last block.
		lastBlockHashBytes := blockchainStore.Get(storage.LastBlockKey)
		lastBlockHash := hex.EncodeToString(lastBlockHashBytes)
		lastBlockBuf := blockchainStore.Get(lastBlockHash)
		if lastBlockBuf != nil {
			var lastBlock types.BlockchainBlock
			_ = lastBlock.Unmarshal(lastBlockBuf)
			prevHash = lastBlock.Hash
		}
		// Extract the post information from the proposed value to hash it.
		postInfo := feed.ParseCustomPaxosValue(msg.Value.CustomValue)
		// Create the block hash.
		blockHash := utils.HashFeedBlock(
			int(msg.Step),
			msg.Value.UniqID,
			postInfo.PostType,
			postInfo.FeedUserID,
			postInfo.PostContentID,
			prevHash,
		)
		// Create the block.
		return types.BlockchainBlock{
			Index:    msg.Step,
			Hash:     blockHash,
			Value:    msg.Value,
			PrevHash: prevHash,
		}
	}
}

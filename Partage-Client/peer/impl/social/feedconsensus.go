package social

import (
	"encoding/hex"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

// FeedBlockchainUpdater takes a user id and returns a paxos feed blockchain updater.
func FeedBlockchainUpdater(userID string, feedStore *feed.Store, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) paxos.BlockchainUpdater {
	return func(newBlock types.BlockchainBlock) {
		utils.PrintDebug("social", "Updating local feed...")
		// Get the blockchain store associated with the user's feed.
		blockchainStore := blockchainStorage.GetStore(feed.IDFromUserID(userID))
		// Update the last block.
		blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		newBlockBytes, _ := newBlock.Marshal()
		// Append the block into the blockchain.
		blockchainStore.Set(newBlockHash, newBlockBytes)
		// Update the feed.
		feedStore.AppendToFeed(blockchainStorage, metadataStore, userID, newBlock)
	}
}

// FeedProposalChecker takes a user id and returns a paxos proposal checker.
func FeedProposalChecker(userID string, feedStore *feed.Store, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		metadata := content.ParseMetadata(msg.Value.CustomValue)
		// Reject the dummy blocks!
		if metadata.Type == content.DUMMY {
			return false
		}
		proposerUserID := metadata.FeedUserID
		// TODO Check remaining credits, timestamp, signature etc.
		_ = feedStore.GetFeedCopy(blockchainStorage, metadataStore, proposerUserID)
		// Self-endorsement, re-endorsement
		// Re-reactions
		// Malformed undo-s
		return true
	}
}

// FeedBlockGenerator takes a user id and returns a paxos feed block generator.
func FeedBlockGenerator(userID string, blockchainStorage storage.MultipurposeStorage) paxos.BlockGenerator {
	return func(msg types.PaxosAcceptMessage) types.BlockchainBlock {
		utils.PrintDebug("social", "Generating feed block...")
		prevHash := make([]byte, 32)
		// Get the blockchain store associated with the user's feed.
		blockchainStore := blockchainStorage.GetStore(feed.IDFromUserID(userID))
		// Get the last block.
		lastBlockHashBytes := blockchainStore.Get(storage.LastBlockKey)
		lastBlockHash := hex.EncodeToString(lastBlockHashBytes)
		lastBlockBuf := blockchainStore.Get(lastBlockHash)
		if lastBlockBuf != nil {
			var lastBlock types.BlockchainBlock
			_ = lastBlock.Unmarshal(lastBlockBuf)
			prevHash = lastBlock.Hash
		}
		// Extract the content metadata from the proposed value to hash it.
		metadata := content.ParseMetadata(msg.Value.CustomValue)
		// Create the block hash.
		blockHash := content.HashMetadata(msg.Step, msg.Value.UniqID, metadata, prevHash)
		// Create the block.
		return types.BlockchainBlock{
			Index:    msg.Step,
			Hash:     blockHash,
			Value:    msg.Value,
			PrevHash: prevHash,
		}
	}
}

func NewFeedConsensusProtocol(userID string, config *peer.Configuration, gossip *gossip.Layer, feedStore *feed.Store) protocol.Protocol {
	protocolID := feed.IDFromUserID(userID)
	return paxos.New(protocolID, config, gossip,
		FeedBlockGenerator(userID, config.BlockchainStorage),
		FeedBlockchainUpdater(userID, feedStore, config.BlockchainStorage, config.BlockchainStorage.GetStore("metadata")),
		FeedProposalChecker(userID, feedStore, config.BlockchainStorage, config.BlockchainStorage.GetStore("metadata")))
}

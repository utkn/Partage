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

func (l *Layer) registerUser(newUserID string) {
	// Load the user feed and add it to the list of known users.
	l.FeedStore.LoadUser(newUserID)
	// Add the appropriate protocol for new blocks proposed by this user.
	protocolID := feed.IDFromUserID(newUserID)
	alreadyExists := l.consensus.IsRegistered(protocolID)
	if !alreadyExists {
		utils.PrintDebug("social", l.GetAddress(), "is registering", newUserID)
		l.consensus.RegisterProtocol(protocolID, l.newFeedConsensusProtocol(newUserID))
	}
	// Update the system size if we are not self-registering.
	if newUserID != l.UserID {
		utils.PrintDebug("social", l.GetAddress(), "is updating system size to", l.consensus.Config.TotalPeers+1)
		l.consensus.UpdateSystemSize(l.Config.TotalPeers + 1)
	}
}

// LoadRegisteredUsers loads the registered users from the registration blockchain.
// Returns the # of users registered.
func (l *Layer) LoadRegisteredUsers(blockchainStorage storage.MultipurposeStorage) int {
	// Get the blocks.
	blocks := utils.LoadBlockchain(blockchainStorage.GetStore("registration"))
	// Now we have a list of registration blocks. Register them one by one.
	for _, block := range blocks {
		c := content.ParseMetadata(block.Value.CustomValue)
		newUserID := c.FeedUserID
		l.registerUser(newUserID)
	}
	return len(blocks)
}

// registrationBlockchainUpdater takes a user id and returns a paxos feed blockchain updater.
func (l *Layer) registrationBlockchainUpdater(blockchainStorage storage.MultipurposeStorage) paxos.BlockchainUpdater {
	return func(newBlock types.BlockchainBlock) {
		utils.PrintDebug("social", "Registering user...")
		// Get the blockchain store associated with the user's feed.
		blockchainStore := blockchainStorage.GetStore("registration")
		// If the block contains a join metadata, then we need to also append to the registration blockchain.
		// Update the last block.
		blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		newBlockBytes, _ := newBlock.Marshal()
		// Append the block into the blockchain.
		blockchainStore.Set(newBlockHash, newBlockBytes)
		// Register the user.
		c := content.ParseMetadata(newBlock.Value.CustomValue)
		newUserID := c.FeedUserID
		l.registerUser(newUserID)
	}
}

// registrationProposalChecker takes a user id and returns a paxos proposal checker.
func (l *Layer) registrationProposalChecker() paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		metadata := content.ParseMetadata(msg.Value.CustomValue)
		// Only allow registration blocks.
		if metadata.Type != content.JOIN {
			return false
		}
		// The user must be unregistered!
		return !l.FeedStore.IsKnown(metadata.FeedUserID)
	}
}

// registrationBlockGenerator takes a user id and returns a paxos feed block generator.
func (l *Layer) registrationBlockGenerator(blockchainStorage storage.MultipurposeStorage) paxos.BlockGenerator {
	return func(msg types.PaxosAcceptMessage) types.BlockchainBlock {
		utils.PrintDebug("social", "Generating registration block...")
		prevHash := make([]byte, 32)
		// Get the blockchain store associated with the user's feed.
		blockchainStore := blockchainStorage.GetStore("registration")
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

func (l *Layer) newRegistrationConsensusProtocol(config *peer.Configuration, gossip *gossip.Layer, feedStore *feed.Store) protocol.Protocol {
	protocolID := "registration"
	return paxos.New(protocolID, config, gossip,
		l.registrationBlockGenerator(config.BlockchainStorage),
		l.registrationBlockchainUpdater(config.BlockchainStorage),
		l.registrationProposalChecker())
}

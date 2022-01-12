package social

import (
	"encoding/hex"
	"fmt"
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
	// Add the user to the list of known users.
	l.FeedStore.AddUser(newUserID)
	// Add the appropriate protocol.
	protocolID := feed.IDFromUserID(newUserID)
	alreadyExists := l.consensus.IsRegistered(protocolID)
	if !alreadyExists {
		utils.PrintDebug("social", l.GetAddress(), "is registering", newUserID)
		l.consensus.RegisterProtocol(protocolID, NewFeedConsensusProtocol(newUserID, l.Config, l.gossip, l.FeedStore))
	}
}

// loadRegisteredUsers loads the registered users from the registration blockchain.
func (l *Layer) loadRegisteredUsers(blockchainStorage storage.MultipurposeStorage) bool {
	// Get the registration blockchain.
	feedBlockchain := blockchainStorage.GetStore("registration")
	// Reconstruct the blockchain.
	lastBlockHashHex := hex.EncodeToString(feedBlockchain.Get(storage.LastBlockKey))
	// If the associated blockchain is completely empty, simply return. New blocks will be added with consensus.
	if lastBlockHashHex == "" {
		return false
	}
	// The first block has its previous hash field set to this value.
	endBlockHasHex := hex.EncodeToString(make([]byte, 32))
	var blocks []types.BlockchainBlock
	// Go back from the last block to the first block.
	for lastBlockHashHex != endBlockHasHex {
		// Get the current last block.
		lastBlockBuf := feedBlockchain.Get(lastBlockHashHex)
		var currBlock types.BlockchainBlock
		err := currBlock.Unmarshal(lastBlockBuf)
		if err != nil {
			fmt.Printf("error during collecting users from registration blockchain: %v\n", err)
			break
		}
		// Prepend into the list of blocks.
		blocks = append([]types.BlockchainBlock{currBlock}, blocks...)
		// Go back.
		lastBlockHashHex = hex.EncodeToString(currBlock.PrevHash)
	}
	// Now we have a list of registration blocks. Register them one by one.
	for _, block := range blocks {
		c := content.ParseMetadata(block.Value.CustomValue)
		newUserID := c.FeedUserID
		l.registerUser(newUserID)
	}
	return true
}

// registrationBlockchainUpdater takes a user id and returns a paxos feed blockchain updater.
func (l *Layer) registrationBlockchainUpdater(feedStore *feed.Store, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) paxos.BlockchainUpdater {
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
func (l *Layer) registrationProposalChecker(feedStore *feed.Store, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		metadata := content.ParseMetadata(msg.Value.CustomValue)
		// Only allow registration blocks.
		if metadata.Type != content.JOIN {
			return false
		}
		// The user must be unregistered!
		return !feedStore.IsKnown(metadata.FeedUserID)
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
		l.registrationBlockchainUpdater(feedStore, config.BlockchainStorage, config.BlockchainStorage.GetStore("metadata")),
		l.registrationProposalChecker(feedStore, config.BlockchainStorage, config.BlockchainStorage.GetStore("metadata")))
}

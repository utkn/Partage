package social

import (
	"encoding/hex"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

// feedBlockchainUpdater takes a user id and returns a paxos feed blockchain updater.
func (l *Layer) feedBlockchainUpdater(userID string) paxos.BlockchainUpdater {
	return func(newBlock types.BlockchainBlock) {
		c := content.ParseMetadata(newBlock.Value.CustomValue)
		utils.PrintDebug("social", l.GetAddress(), " is updating its local feed with", c)
		// Get the blockchain store associated with the user's feed.
		blockchainStore := l.FeedStore.BlockchainStorage.GetStore(feed.IDFromUserID(userID))
		// If the block contains a join metadata, then we need to also append to the registration blockchain.
		// Update the last block.
		blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		newBlockBytes, _ := newBlock.Marshal()
		// Append the block into the blockchain.
		blockchainStore.Set(newBlockHash, newBlockBytes)
		// Update the feed.
		l.FeedStore.AppendToFeed(userID, newBlock)
	}
}

// feedProposalChecker takes a user id and returns a paxos proposal checker.
func (l *Layer) feedProposalChecker(userID string) paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		metadata := content.ParseMetadata(msg.Value.CustomValue)
		// Reject the dummy blocks!
		if metadata.Type == content.DUMMY {
			return false
		}
		isValid := l.FeedStore.IsValidMetadata(metadata)
		utils.PrintDebug("social", l.GetAddress(), " has checked a proposal. Result =", isValid)
		return isValid
		// TODO timestamp, signature etc.
		// Check remaining credits ... DONE
		// Self-endorsement, re-endorsement ... DONE
		// Re-reactions ... DONE
		// Malformed undo-s ... DONE
	}
}

// feedBlockGenerator takes a user id and returns a paxos feed block generator.
func (l *Layer) feedBlockGenerator(userID string) paxos.BlockGenerator {
	return func(msg types.PaxosAcceptMessage) types.BlockchainBlock {
		utils.PrintDebug("social", l.GetAddress(), "is generating a feed block...")
		prevHash := make([]byte, 32)
		// Get the blockchain store associated with the user's feed.
		blockchainStore := l.FeedStore.BlockchainStorage.GetStore(feed.IDFromUserID(userID))
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

// newFeedConsensusProtocol generates a new feed consensus protocol for the given user.
func (l *Layer) newFeedConsensusProtocol(userID string) protocol.Protocol {
	protocolID := feed.IDFromUserID(userID)
	return paxos.New(protocolID, l.Config, l.gossip,
		l.feedBlockGenerator(userID),
		l.feedBlockchainUpdater(userID),
		l.feedProposalChecker(userID))
}

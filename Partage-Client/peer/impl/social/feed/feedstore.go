package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Store struct {
	sync.RWMutex
	feedMap         map[string]*Feed
	reactionHandler *ReactionHandler
}

func LoadStore() *Store {
	return &Store{
		feedMap:         make(map[string]*Feed),
		reactionHandler: NewReactionHandler(),
	}
}

// GetFeed loads the feed of the user associated with the given id. The feed is loaded from the blockchain storage.
func (s *Store) GetFeed(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string) *Feed {
	s.RLock()
	feed, ok := s.feedMap[userID]
	s.RUnlock()
	if !ok {
		s.Lock()
		// Load the feed from the blockchain into the in-memory storage.
		s.loadFeedFromBlockchain(blockchainStorage, metadataStore, userID)
		feed = s.feedMap[userID]
		s.Unlock()
	}
	return feed
}

// GetFeedCopy loads the feed of the user associated with the given id. The feed is loaded from the blockchain storage.
// The returned feed is a copied instance.
func (s *Store) GetFeedCopy(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string) *Feed {
	return s.GetFeed(blockchainStorage, metadataStore, userID).Copy()
}

// GetReactions returns the known reactions associated with the given content id.
func (s *Store) GetReactions(contentID string) []content.ReactionInfo {
	return s.reactionHandler.GetReactionsCopy(contentID)
}

// GetRegisteredUsers returns the set of users that were registered with this feed store.
func (s *Store) GetRegisteredUsers() map[string]struct{} {
	s.RLock()
	defer s.RUnlock()
	userSet := make(map[string]struct{})
	for userID := range s.feedMap {
		userSet[userID] = struct{}{}
	}
	return userSet
}

// loadFeedFromBlockchain loads the feed associated with the given user id from the blockchain storage.
// Warning: Not thread-safe.
func (s *Store) loadFeedFromBlockchain(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string) {
	// Get the feed blockchain associated with the given user id.
	feedBlockchain := blockchainStorage.GetStore(IDFromUserID(userID))
	// Construct the feed blockchain.
	lastBlockHashHex := hex.EncodeToString(feedBlockchain.Get(storage.LastBlockKey))
	// If the associated blockchain is completely empty, save an empty feed blockchain.
	if lastBlockHashHex == "" {
		s.feedMap[userID] = NewEmptyFeed(userID, metadataStore)
		return
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
			fmt.Printf("Error during collecting feedBlockchain from blockchain %v\n", err)
			continue
		}
		// Prepend into the list of blocks.
		blocks = append([]types.BlockchainBlock{currBlock}, blocks...)
		// Go back.
		lastBlockHashHex = hex.EncodeToString(currBlock.PrevHash)
	}
	// Now we have a list of blocks. Add them one by one.
	for _, block := range blocks {
		s.AppendToFeed(blockchainStorage, metadataStore, userID, block)
	}
}

// AppendToFeed updates the feed associated with the given user id with the given new block.
func (s *Store) AppendToFeed(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string, newBlock types.BlockchainBlock) {
	// Extract the content metadata.
	c := content.ParseMetadata(newBlock.Value.CustomValue)
	// --- Append into the in-memory as well.
	// Get the associated feed.
	f := s.GetFeed(blockchainStorage, metadataStore, userID)
	// First, save the metadata into the metadata storage.
	if c.ContentID != "" {
		metadataBytes := content.UnparseMetadata(c)
		f.metadataStore.Set(c.ContentID, metadataBytes)
	}
	blockHash := hex.EncodeToString(newBlock.Hash)
	// Append into the actual feed & update the user state.
	err := f.Append(c, blockHash)
	if err != nil {
		fmt.Println(err)
		return
	}
	// If we have an endorsement block, then we need to update the endorsed user's state explicitly.
	if c.Type == content.ENDORSEMENT {
		// Extract the endorsed user.
		endorsedID, err := content.ParseEndorsedUserID(c)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Update the endorsed user's state.
		s.GetFeed(blockchainStorage, metadataStore, endorsedID).UpdateEndorsement(c)
	}
	// If we have a reaction block, then we need to inform the reaction handler.
	if c.Type == content.REACTION {
		// Extract the reaction from metadata.
		reaction, err := content.ParseReactionMetadata(c)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Save the reaction.
		s.reactionHandler.SaveReaction(reaction, c.RefContentID, c.FeedUserID)
	}
	// If we have an undo block, we need to do some special stuff.
	if c.Type == content.UNDO {
		// Extract the referred block hash.
		refBlock, err := content.ParseUndoMetadata(c)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Get the referred metadata.
		referredMetadata, err := f.GetWithHash(refBlock)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Try to undo the feed.
		err = f.Undo(referredMetadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Undo the reaction.
		if referredMetadata.Type == content.REACTION {
			s.reactionHandler.UndoReaction(referredMetadata.RefContentID, c.FeedUserID)
		}
	}
}

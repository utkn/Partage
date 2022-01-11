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
		// Load the feed from the blockchain.
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
func (s *Store) loadFeedFromBlockchain(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string) {
	// Get the feed blockchain associated with the given user id.
	feedBlockchain := blockchainStorage.GetStore(IDFromUserID(userID))
	// Construct the feed blockchain.
	lastBlockHashHex := hex.EncodeToString(feedBlockchain.Get(storage.LastBlockKey))
	// If the associated blockchain is completely empty, return an empty feedBlockchain.
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
	// Create the feed.
	feed := NewEmptyFeed(userID, metadataStore)
	// Now we have a list of blocks.
	for _, block := range blocks {
		metadata := content.ParseMetadata(block.Value.CustomValue)
		// No need to acquire the lock because there are no other references to this feed yet.
		s.processAndAppend(feed, blockchainStorage, metadataStore, userID, metadata, hex.EncodeToString(block.Hash))
	}
	// Save the feed.
	s.feedMap[userID] = feed
}

// VERY IMPORTANT FUNCTION
func (s *Store) processAndAppend(f *Feed, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string, c content.Metadata, blockHash string) {
	// First, save the metadata into the metadata storage.
	if c.ContentID != "" {
		metadataBytes := content.UnparseMetadata(c)
		f.metadataStore.Set(c.ContentID, metadataBytes)
	}
	// Append into the feed.
	f.Append(c, blockHash)
	// Update the feed's user state.
	err := f.userState.Update(c)
	if err != nil {
		fmt.Printf("error while updating user state %s\n", err)
	}
	// If we have an endorsement block, then we need to update the associated user state manually.
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
		// Undo the user state.
		err = f.userState.Undo(referredMetadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Undo the reaction.
		if referredMetadata.Type == content.REACTION {
			s.reactionHandler.UndoReaction(referredMetadata.RefContentID, c.FeedUserID)
		}
		// Undo the text or comment.
		if referredMetadata.Type == content.TEXT || referredMetadata.Type == content.COMMENT {
			f.HideContent(referredMetadata.ContentID)
		}
	}
}

// UpdateFeed updates the blockchain associated with the given user id with the given new block. The new block is also
// added to the in-memory storage.
func (s *Store) UpdateFeed(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store, userID string, newBlock types.BlockchainBlock) {
	// Get the blockchain store associated with the user's feed.
	blockchainStore := blockchainStorage.GetStore(IDFromUserID(userID))
	// Update the last block.
	blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
	newBlockHash := hex.EncodeToString(newBlock.Hash)
	newBlockBytes, _ := newBlock.Marshal()
	// Append the block into the blockchain.
	blockchainStore.Set(newBlockHash, newBlockBytes)
	// Extract the content metadata.
	metadata := content.ParseMetadata(newBlock.Value.CustomValue)
	// Append into the in-memory as well.
	s.processAndAppend(s.GetFeed(blockchainStorage, metadataStore, userID),
		blockchainStorage, metadataStore, userID, metadata, newBlockHash)

}

package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Store struct {
	sync.RWMutex
	feedMap         map[string]*Feed
	knownUsers      map[string]struct{}
	reactionHandler *ReactionHandler

	BlockchainStorage storage.MultipurposeStorage
	MetadataStore     storage.Store
}

func LoadStore(blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) *Store {
	return &Store{
		feedMap:           make(map[string]*Feed),
		knownUsers:        make(map[string]struct{}),
		reactionHandler:   NewReactionHandler(),
		BlockchainStorage: blockchainStorage,
		MetadataStore:     metadataStore,
	}
}

// loadFeed loads the feed associated with the given user id from the blockchain storage into memory.
// Warning: thread-unsafe
func (s *Store) loadFeed(userID string) {
	store := s.BlockchainStorage.GetStore(IDFromUserID(userID))
	blocks := utils.LoadBlockchain(store)
	// Create an empty feed.
	s.feedMap[userID] = NewEmptyFeed(userID, s.MetadataStore)
	// Move into memory.
	for _, block := range blocks {
		s.appendToFeed(userID, block)
	}
}

// getFeed returns the feed associated with the given user id.
// Warning: thread-unsafe
func (s *Store) getFeed(userID string) *Feed {
	feed, _ := s.feedMap[userID]
	return feed
}

// GetFeedCopy loads the feed of the user associated with the given id. The feed is loaded from the blockchain storage.
// The returned feed is a copied instance.
func (s *Store) GetFeedCopy(userID string) *Feed {
	s.RLock()
	defer s.RUnlock()
	feed := s.getFeed(userID)
	if feed == nil {
		return nil
	}
	return feed.Copy()
}

// GetReactions returns the known reactions associated with the given content id.
func (s *Store) GetReactions(contentID string) []ReactionInfo {
	return s.reactionHandler.GetReactionsCopy(contentID)
}

// GetKnownUsers returns the set of users that were registered with this feed store.
func (s *Store) GetKnownUsers() map[string]struct{} {
	s.RLock()
	defer s.RUnlock()
	userSet := make(map[string]struct{})
	for userID := range s.knownUsers {
		userSet[userID] = struct{}{}
	}
	return userSet
}

// LoadUser adds the given user id to this feed store and loads the feed from storage.
// Should be invoked during user registration.
func (s *Store) LoadUser(userID string) {
	s.Lock()
	defer s.Unlock()
	// First save in the known users set.
	s.knownUsers[userID] = struct{}{}
	// Then, load the feed from the storage.
	s.loadFeed(userID)
}

// IsKnown returns true if the given user id was added with LoadUser to this feed store.
func (s *Store) IsKnown(userID string) bool {
	s.RLock()
	defer s.RUnlock()
	_, isKnown := s.knownUsers[userID]
	return isKnown
}

// AppendToFeed updates the feed state associated with the given user id with the given new block.
func (s *Store) AppendToFeed(userID string, newBlock types.BlockchainBlock) {
	s.Lock()
	defer s.Unlock()
	s.appendToFeed(userID, newBlock)
}

// Thread unsafe version of AppendToFeed.
func (s *Store) appendToFeed(userID string, newBlock types.BlockchainBlock) {
	// Extract the content metadata.
	metadata := content.ParseMetadata(newBlock.Value.CustomValue)
	// --- Append into the in-memory as well.
	// Get the associated feed.
	feed := s.getFeed(userID)
	// First, save the metadata into the metadata storage.
	if metadata.ContentID != "" {
		metadataBytes := content.UnparseMetadata(metadata)
		s.MetadataStore.Set(metadata.ContentID, metadataBytes)
	}
	blockHash := hex.EncodeToString(newBlock.Hash)
	// Append into the actual feed & update the user state.
	feedContent, err := feed.Append(metadata, blockHash)
	if err != nil {
		fmt.Println(err)
		return
	}
	// If we have a follow block, inform the followed user.
	if metadata.Type == content.FOLLOW {
		followedUserID, err := content.ParseFollowedUser(metadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Update the followed user's state.
		s.getFeed(followedUserID).AddFollower(metadata.FeedUserID)
	}
	// If we have an endorsement block, then we need to update the endorsed user's state explicitly.
	if metadata.Type == content.ENDORSEMENT {
		// Extract the endorsed user.
		endorsedID, err := content.ParseEndorsedUserID(metadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Update the endorsed user's state.
		s.getFeed(endorsedID).ReceiveEndorsement(metadata)
	}
	// If we have a reaction block, then we need to inform the reaction handler.
	if metadata.Type == content.REACTION {
		// Extract the reaction from metadata.
		reaction, err := content.ParseReactionMetadata(metadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Save the reaction.
		s.reactionHandler.SaveReaction(feedContent, reaction)
	}
	// If we have an undo block, we need to do some special stuff.
	if metadata.Type == content.UNDO {
		// Extract the referred block hash.
		refBlock, err := content.ParseUndoMetadata(metadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Get the referred metadata.
		referredMetadata, err := feed.GetWithHash(refBlock)
		if err != nil {
			fmt.Println(err)
			return
		}
		// (1) Try to apply the undo to the feed.
		err = feed.Undo(referredMetadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// (2) Try to undo the follow from the followed user.
		if referredMetadata.Type == content.FOLLOW {
			followedUserID, _ := content.ParseFollowedUser(referredMetadata)
			s.getFeed(followedUserID).RemoveFollower(referredMetadata.FeedUserID)
		}
		// (3) Try to undo a reaction.
		if referredMetadata.Type == content.REACTION {
			s.reactionHandler.UndoReaction(referredMetadata.RefContentID, metadata.FeedUserID)
		}
	}
}

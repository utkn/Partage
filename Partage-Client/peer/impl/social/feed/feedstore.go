package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sort"
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
	_, ok := s.feedMap["feed"]
	if utils.GLOBAL_FEED && ok {
		return
	}
	store := s.BlockchainStorage.GetStore(IDFromUserID(userID))
	blocks := utils.LoadBlockchain(store)
	// Create an empty feed.
	feedName := userID
	if utils.GLOBAL_FEED {
		feedName = "feed"
	}
	s.feedMap[feedName] = NewEmptyFeed(userID, s.MetadataStore)
	// Move into memory.
	for _, block := range blocks {
		s.appendToFeed(userID, block)
	}
}

// getFeed returns the feed associated with the given user id.
// Warning: thread-unsafe
func (s *Store) getFeed(userID string) *Feed {
	feedName := userID
	if utils.GLOBAL_FEED {
		feedName = "feed"
	}
	feed, _ := s.feedMap[feedName]
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
	return s.getKnownUsers()
}

// Thread-unsafe version of GetKnownUsers.
func (s *Store) getKnownUsers() map[string]struct{} {
	userSet := make(map[string]struct{})
	for userID := range s.knownUsers {
		userSet[userID] = struct{}{}
	}
	return userSet
}

// QueryContents returns all the known contents from the given filters, sorted by their metadata timestamp.
func (s *Store) QueryContents(filter content.Filter) []Content {
	s.RLock()
	defer s.RUnlock()
	// User selectors.
	selectedUsers := filter.OwnerIDs
	if selectedUsers == nil {
		for u := range s.getKnownUsers() {
			selectedUsers = append(selectedUsers, u)
		}
	}
	selectedTypes := make(map[content.Type]struct{}, len(filter.Types))
	for _, t := range filter.Types {
		selectedTypes[t] = struct{}{}
	}
	var filtered []Content
	for _, user := range selectedUsers {
		userFeed := s.getFeed(user)
		// If the user does not exist, skip him.
		if userFeed == nil {
			utils.PrintDebug("social", "feed.Store.QueryContent: ", user, "does not exist")
			continue
		}
		contents := userFeed.GetContents()
		for _, c := range contents {
			if filter.Match(c.Metadata) {
				filtered = append(filtered, c)
			}
		}
	}
	// Sort by the timestamp.
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Timestamp < filtered[j].Timestamp
	})
	return filtered
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
		// Get the referred content.
		referredContent, err := feed.GetWithHash(refBlock)
		if err != nil {
			fmt.Println(err)
			return
		}
		// (1) Try to apply the undo to the feed.
		err = feed.Undo(referredContent.Metadata)
		if err != nil {
			fmt.Println(err)
			return
		}
		// (2) Try to undo the follow from the followed user.
		if referredContent.Type == content.FOLLOW {
			followedUserID, _ := content.ParseFollowedUser(referredContent.Metadata)
			s.getFeed(followedUserID).RemoveFollower(referredContent.FeedUserID)
		}
		// (3) Try to undo a reaction.
		if referredContent.Type == content.REACTION {
			s.reactionHandler.UndoReaction(referredContent.RefContentID, metadata.FeedUserID)
		}
	}
}

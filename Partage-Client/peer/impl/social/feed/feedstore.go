package feed

import (
	"encoding/hex"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Store struct {
	sync.RWMutex
	feedMap map[string]*Feed
}

func LoadStore() *Store {
	return &Store{
		feedMap: make(map[string]*Feed),
	}
}

// GetFeed loads the feed of the user associated with the given id. The feed is loaded from the blockchain storage.
func (s *Store) GetFeed(blockchainStorage storage.MultipurposeStorage, userID string) *Feed {
	s.RLock()
	feed, ok := s.feedMap[userID]
	s.RUnlock()
	if !ok {
		s.Lock()
		// Load the feed from the blockchain.
		s.feedMap[userID] = LoadFeedFromBlockchain(blockchainStorage, userID)
		feed = s.feedMap[userID]
		s.Unlock()
	}
	return feed
}

// GetRegisteredUsers returns the set of users that were registered with this feed store.
func (s *Store) GetRegisteredUsers() map[string]struct{} {
	s.RLock()
	defer s.RUnlock()
	userSet := make(map[string]struct{})
	for userID, _ := range s.feedMap {
		userSet[userID] = struct{}{}
	}
	return userSet
}

// UpdateFeed updates the blockchain associated with the given user id with the given new block. The new block is also
// added to the in-memory storage.
func (s *Store) UpdateFeed(blockchainStorage storage.MultipurposeStorage, userID string, newBlock types.BlockchainBlock) {
	// Get the blockchain store associated with the user's feed.
	blockchainStore := blockchainStorage.GetStore(IDFromUserID(userID))
	// Update the last block.
	blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
	newBlockHash := hex.EncodeToString(newBlock.Hash)
	newBlockBytes, _ := newBlock.Marshal()
	// Append the block into the blockchain.
	blockchainStore.Set(newBlockHash, newBlockBytes)
	// Extract the post info.
	postInfo := ParseCustomPaxosValue(newBlock.Value.CustomValue)
	// Append into the in-memory as well.
	s.GetFeed(blockchainStorage, userID).Append(LoadFeedContentFromPostInfo(postInfo, false))
}

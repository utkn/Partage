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

func (s *Store) UpdateFeed(blockchainStorage storage.MultipurposeStorage, userID string, newBlock types.BlockchainBlock) {
	// Get the blockchain store associated with the user's feed.
	blockchainStore := blockchainStorage.GetStore(FeedIDFromUserID(userID))
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

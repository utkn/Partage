package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sync"
)

// Feed represents a user's feed.
type Feed struct {
	sync.RWMutex
	contents []FeedContent
	UserID   string
	//CurrentCredits int
}

func NewEmptyFeed(userID string) *Feed {
	return &Feed{
		contents: []FeedContent{},
		UserID:   userID,
	}
}

func (f *Feed) GetContents() []FeedContent {
	f.RLock()
	defer f.RUnlock()
	var contents []FeedContent
	for _, c := range f.contents {
		contents = append(contents, c)
	}
	return contents
}

// Append appends a new feed content into the feed. The associated blockchain is not modified.
func (f *Feed) Append(c FeedContent) {
	f.Lock()
	defer f.Unlock()
	f.contents = append(f.contents, c)
}

// LoadFeedFromBlockchain loads the feed associated with the given user id from the blockchain storage.
func LoadFeedFromBlockchain(blockchainStorage storage.MultipurposeStorage, userID string) *Feed {
	// Get the feed blockchain associated with the given user id.
	feed := blockchainStorage.GetStore(IDFromUserID(userID))
	// Construct the feed.
	lastBlockHashHex := hex.EncodeToString(feed.Get(storage.LastBlockKey))
	// If the associated blockchain is completely empty, return an empty feed.
	if lastBlockHashHex == "" {
		return NewEmptyFeed(userID)
	}
	// The first block has its previous hash field set to this value.
	endBlockHasHex := hex.EncodeToString(make([]byte, 32))
	var blocks []types.BlockchainBlock
	// Go back from the last block to the first block.
	for lastBlockHashHex != endBlockHasHex {
		// Get the current last block.
		lastBlockBuf := feed.Get(lastBlockHashHex)
		var currBlock types.BlockchainBlock
		err := currBlock.Unmarshal(lastBlockBuf)
		if err != nil {
			fmt.Printf("Error during collecting feed from blockchain %v\n", err)
			continue
		}
		// Prepend into the list of blocks.
		blocks = append([]types.BlockchainBlock{currBlock}, blocks...)
		// Go back.
		lastBlockHashHex = hex.EncodeToString(currBlock.PrevHash)
	}
	// Now we have a list of blocks. Convert them into a list of feed contents.
	var contents []FeedContent
	for _, block := range blocks {
		postInfo := ParseCustomPaxosValue(block.Value.CustomValue)
		contents = append(contents, LoadFeedContentFromPostInfo(postInfo, false))
	}
	// Return the feed.
	return &Feed{
		UserID:   userID,
		contents: contents,
	}
}

func IDFromUserID(userID string) string {
	return "feed-" + userID
}

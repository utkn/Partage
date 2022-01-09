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
	sync.Mutex
	Contents []FeedContent
	UserID   string
	//CurrentCredits int
}

// Append appends a new feed content into the feed. The associated blockchain is not modified.
func (f *Feed) Append(c FeedContent) {
	f.Lock()
	defer f.Unlock()
	f.Contents = append(f.Contents, c)
}

// LoadFeedFromBlockchain loads the feed associated with the given user id from the blockchain storage.
func LoadFeedFromBlockchain(blockchainStorage storage.MultipurposeStorage, userID string) *Feed {
	// Get the feed blockchain associated with the given user id.
	feed := blockchainStorage.GetStore(FeedIDFromUserID(userID))
	// Construct the feed.
	lastBlockHashHex := hex.EncodeToString(feed.Get(storage.LastBlockKey))
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
		Contents: contents,
	}
}

func FeedIDFromUserID(userID string) string {
	return "feed-" + userID
}

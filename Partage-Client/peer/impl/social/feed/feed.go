package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/social/feed/content"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"sync"
)

// Feed represents a user's feed.
type Feed struct {
	sync.RWMutex
	userState *UserState
	contents  []content.Metadata
	UserID    string
}

func NewEmptyFeed(userID string) *Feed {
	return &Feed{
		userState: NewInitialUserState(userID),
		contents:  []content.Metadata{},
		UserID:    userID,
	}
}

func (f *Feed) Copy() *Feed {
	f.RLock()
	defer f.RUnlock()
	userState := f.userState.Copy()
	var contents []content.Metadata
	for _, c := range f.contents {
		contents = append(contents, c)
	}
	return &Feed{
		userState: &userState,
		contents:  contents,
		UserID:    f.UserID,
	}
}

func (f *Feed) GetUserStateCopy() UserState {
	f.RLock()
	defer f.RUnlock()
	return f.userState.Copy()
}

func (f *Feed) GetContents() []content.Metadata {
	f.RLock()
	defer f.RUnlock()
	var contents []content.Metadata
	for _, c := range f.contents {
		contents = append(contents, c)
	}
	return contents
}

// Append appends a new feed content into the feed. The associated blockchain is not modified.
func (f *Feed) Append(c content.Metadata) {
	f.Lock()
	defer f.Unlock()
	f.processAndAppend(c)
}

func (f *Feed) processAndAppend(c content.Metadata) {
	// Process the metadata by updating the user state.
	err := f.userState.Update(c)
	if err != nil {
		fmt.Printf("error while updating user state %s\n", err)
	}
	// Append the metadata.
	f.contents = append(f.contents, c)
}

// UpdateEndorsement updates the endorsement given by a different user.
func (f *Feed) UpdateEndorsement(endorsement content.Metadata) {
	f.Lock()
	defer f.Unlock()
	endorserID := endorsement.FeedUserID
	complete := f.userState.Endorsement.Update(endorsement.Timestamp, endorserID)
	// If the endorsement request was fulfilled by the network, reward the user.
	if complete {
		f.userState.CurrentCredits += ENDORSEMENT_REWARD
	}
}

// LoadFeedFromBlockchain loads the feed associated with the given user id from the blockchain storage.
func LoadFeedFromBlockchain(blockchainStorage storage.MultipurposeStorage, userID string) *Feed {
	// Get the feed blockchain associated with the given user id.
	feedBlockchain := blockchainStorage.GetStore(IDFromUserID(userID))
	// Construct the feed blockchain.
	lastBlockHashHex := hex.EncodeToString(feedBlockchain.Get(storage.LastBlockKey))
	// If the associated blockchain is completely empty, return an empty feedBlockchain.
	if lastBlockHashHex == "" {
		return NewEmptyFeed(userID)
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
	feed := NewEmptyFeed(userID)
	// Now we have a list of blocks. Append them into the feed one by one.
	for _, block := range blocks {
		postInfo := content.ParseCustomPaxosValue(block.Value.CustomValue)
		// No need to acquire the lock because there are no other references to this feed yet.
		feed.processAndAppend(postInfo)
	}
	// Return the feed.
	return feed
}

func IDFromUserID(userID string) string {
	return "feed-" + userID
}

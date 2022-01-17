package feed

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"sync"
)

// Content represents a content on a feed, which is a metadata + its block hash.
type Content struct {
	content.Metadata
	BlockHash string
}

// Feed represents a user's feed.
type Feed struct {
	sync.RWMutex
	UserID      string
	userState   *UserState
	contents    []Content
	blockHashes map[string]Content
	// Undo-ed contents.
	hiddenContentIDs map[string]struct{}
	metadataStore    storage.Store
}

func NewEmptyFeed(userID string, metadataStore storage.Store) *Feed {
	return &Feed{
		UserID:           userID,
		userState:        NewInitialUserState(userID),
		contents:         []Content{},
		blockHashes:      make(map[string]Content),
		hiddenContentIDs: make(map[string]struct{}),
		metadataStore:    metadataStore,
	}
}

func (f *Feed) Copy() *Feed {
	f.RLock()
	defer f.RUnlock()
	userState := f.userState.Copy()
	var contents []Content
	for _, c := range f.contents {
		contents = append(contents, c)
	}
	blockHashes := make(map[string]Content, len(f.blockHashes))
	for k, v := range f.blockHashes {
		blockHashes[k] = v
	}
	hiddenContentIDs := make(map[string]struct{}, len(f.hiddenContentIDs))
	for k, v := range f.hiddenContentIDs {
		hiddenContentIDs[k] = v
	}
	return &Feed{
		UserID:           f.UserID,
		userState:        &userState,
		contents:         contents,
		blockHashes:      blockHashes,
		hiddenContentIDs: hiddenContentIDs,
		// The store cannot be copied!
		metadataStore: f.metadataStore,
	}
}

func (f *Feed) GetUserStateCopy() UserState {
	f.RLock()
	defer f.RUnlock()
	return f.userState.Copy()
}

func (f *Feed) GetContents() []Content {
	f.RLock()
	defer f.RUnlock()
	var contents []Content
	for _, c := range f.contents {
		_, hidden := f.hiddenContentIDs[c.ContentID]
		// Hide the content id.
		if hidden {
			c.ContentID = ""
		}
		contents = append(contents, c)
	}
	return contents
}

// GetWithHash returns the metadata associated with the given block hash.
func (f *Feed) GetWithHash(blockHash string) (Content, error) {
	f.RLock()
	defer f.RUnlock()
	m, ok := f.blockHashes[blockHash]
	if !ok {
		return Content{}, fmt.Errorf("feed: unknown block hash")
	}
	return m, nil
}

// Append appends a new feed content into the feed and updates the user state accordingly. The underlying blockchain is not modified.
// Returns the appended content.
func (f *Feed) Append(metadata content.Metadata, blockHash string) (Content, error) {
	f.Lock()
	defer f.Unlock()
	// Create the feed metadata.
	c := Content{
		Metadata:  metadata,
		BlockHash: blockHash,
	}
	// Add the content.
	f.contents = append(f.contents, c)
	f.blockHashes[blockHash] = c
	return c, f.userState.Update(metadata)
}

// Undo tries to undo the given already appended metadata. The underlying blockchain is not modified.
func (f *Feed) Undo(metadata content.Metadata) error {
	f.Lock()
	defer f.Unlock()
	// First, undo the user state if possible.
	err := f.userState.Undo(metadata)
	if err != nil {
		return err
	}
	// Then, try to undo the feed.
	if metadata.Type == content.TEXT || metadata.Type == content.COMMENT {
		f.hiddenContentIDs[metadata.ContentID] = struct{}{}
	}
	return nil
}

// ReceiveEndorsement updates the endorsement given by a different user.
func (f *Feed) ReceiveEndorsement(endorsement content.Metadata) {
	f.Lock()
	defer f.Unlock()
	endorserID := endorsement.FeedUserID
	complete := f.userState.EndorsementHandler.ReceiveEndorsement(endorsement.Timestamp, endorserID)
	// If the endorsement request was fulfilled by the network, reward the user.
	if complete {
		f.userState.CurrentCredits += ENDORSEMENT_REWARD
	}
}

func (f *Feed) AddFollower(followerID string) {
	f.Lock()
	defer f.Unlock()
	f.userState.AddFollower(followerID)
}

func (f *Feed) RemoveFollower(followerID string) {
	f.Lock()
	defer f.Unlock()
	f.userState.RemoveFollower(followerID)
}

func IDFromUserID(userID string) string {
	if utils.GLOBAL_FEED {
		return "feed"
	}
	return "feed-" + userID
}

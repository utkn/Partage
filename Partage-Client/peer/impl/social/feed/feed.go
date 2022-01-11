package feed

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/storage"
	"sync"
)

// Feed represents a user's feed.
type Feed struct {
	sync.RWMutex
	UserID      string
	userState   *UserState
	contents    []content.Metadata
	blockHashes map[string]content.Metadata
	// Undo-ed contents.
	hiddenContentIDs map[string]struct{}
	metadataStore    storage.Store
}

func NewEmptyFeed(userID string, metadataStore storage.Store) *Feed {
	return &Feed{
		UserID:           userID,
		userState:        NewInitialUserState(userID),
		contents:         []content.Metadata{},
		blockHashes:      make(map[string]content.Metadata),
		hiddenContentIDs: make(map[string]struct{}),
		metadataStore:    metadataStore,
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
	blockHashes := make(map[string]content.Metadata, len(f.blockHashes))
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

func (f *Feed) GetContents() []content.Metadata {
	f.RLock()
	defer f.RUnlock()
	var contents []content.Metadata
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
func (f *Feed) GetWithHash(blockHash string) (content.Metadata, error) {
	f.RLock()
	defer f.RUnlock()
	m, ok := f.blockHashes[blockHash]
	if !ok {
		return content.Metadata{}, fmt.Errorf("feed: unknown block hash")
	}
	return m, nil
}

// Append appends a new feed content into the feed and updates the user state accordingly. The underlying blockchain is not modified.
func (f *Feed) Append(c content.Metadata, blockHash string) error {
	f.Lock()
	defer f.Unlock()
	// Add the metadata.
	f.contents = append(f.contents, c)
	f.blockHashes[blockHash] = c
	return f.userState.Update(c)
}

// Undo tries to undo the given already appended metadata. The underlying blockchain is not modified.
func (f *Feed) Undo(c content.Metadata) error {
	f.Lock()
	defer f.Unlock()
	// First, undo the user state if possible.
	err := f.userState.Undo(c)
	if err != nil {
		return err
	}
	// Then, try to undo the feed.
	if c.Type == content.TEXT || c.Type == content.COMMENT {
		f.hiddenContentIDs[c.ContentID] = struct{}{}
	}
	return nil
}

// UpdateEndorsement updates the endorsement given by a different user.
func (f *Feed) UpdateEndorsement(endorsement content.Metadata) {
	f.Lock()
	defer f.Unlock()
	endorserID := endorsement.FeedUserID
	complete := f.userState.EndorsementHandler.Update(endorsement.Timestamp, endorserID)
	// If the endorsement request was fulfilled by the network, reward the user.
	if complete {
		f.userState.CurrentCredits += ENDORSEMENT_REWARD
	}
}

func IDFromUserID(userID string) string {
	return "feed-" + userID
}

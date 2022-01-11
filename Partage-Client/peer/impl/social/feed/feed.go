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
	UserID        string
	userState     *UserState
	contents      []content.Metadata
	blockHashes   map[string]content.Metadata
	metadataStore storage.Store
}

func NewEmptyFeed(userID string, metadataStore storage.Store) *Feed {
	return &Feed{
		UserID:        userID,
		userState:     NewInitialUserState(userID),
		contents:      []content.Metadata{},
		blockHashes:   make(map[string]content.Metadata),
		metadataStore: metadataStore,
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
	return &Feed{
		UserID:      f.UserID,
		userState:   &userState,
		contents:    contents,
		blockHashes: blockHashes,
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

// Append appends a new feed content into the feed without processing it. The associated blockchain is not modified.
func (f *Feed) Append(c content.Metadata, blockHash string) {
	f.Lock()
	defer f.Unlock()
	// Add the metadata.
	f.contents = append(f.contents, c)
	f.blockHashes[blockHash] = c
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

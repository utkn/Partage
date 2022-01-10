package peer

import (
	"go.dedis.ch/cs438/peer/impl/data/contentfilter"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/social/feed/content"
	"go.dedis.ch/cs438/transport"
	"io"
)

type PartageClient interface {
	RegisterUser() error
	// SharePost shares the post with the given content and returns the content ID.
	SharePost(data io.Reader) (string, error)
	// DownloadPost fetches the post with the given content id from the network.
	DownloadPost(contentID string) ([]byte, error)
	// DiscoverContent returns the matched content ids.
	DiscoverContent(filter contentfilter.ContentFilter) ([]string, error)
	SharePrivatePost(msg transport.Message, recipients [][32]byte) error
	UpdateFeed(info content.Metadata) error
	GetHashedPublicKey() [32]byte
	GetUserID() string
	GetKnownUsers() map[string]struct{}
	GetFeedContents(userID string) []content.Metadata
	GetUserState(userID string) feed.UserState
	BlockUser(publicKeyHash [32]byte)
	UnblockUser(publicKeyHash [32]byte)
}

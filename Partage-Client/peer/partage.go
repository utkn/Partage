package peer

import (
	"go.dedis.ch/cs438/peer/impl/data/contentfilter"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/social/feed/content"
	"go.dedis.ch/cs438/transport"
)

type PartageClient interface {
	RegisterUser() error
	// ShareTextPost shares the text post with the given content and returns the content ID.
	ShareTextPost(content string) (string, error)
	DownloadPost(contentID string) ([]byte, error)
	// SearchContent returns the matched content ids.
	SearchContent(filter contentfilter.ContentFilter) ([]string, error)
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

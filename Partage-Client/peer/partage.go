package peer

import (
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/transport"
)

type PartageClient interface {
	RegisterUser() error
	SharePost(content string) error
	SharePrivatePost(msg transport.Message, recipients [][32]byte) error
	UpdateFeed(info feed.PostInfo) error
	GetHashedPublicKey() [32]byte
	GetUserID() string
	GetKnownUsers() map[string]struct{}
	GetSharedPosts(userID string) []feed.FeedContent
	BlockUser(publicKeyHash [32]byte)
	UnblockUser(publicKeyHash [32]byte)
}

package peer

import (
	"crypto/rsa"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
)

type SocialPeer interface {
	RegisterUser() error
	// ShareDownloadableContent shares the given content into the network and returns the generated metadata and its block hash.
	ShareDownloadableContent(post content.PrivateContent, p content.Type) (content.Metadata, string, error)
	// DownloadContent fetches the post with the given content id from the network.
	DownloadContent(contentID string) ([]byte, error)
	// QueryFeedContents queries the feed store and returns all the matching contents from the stored blockchains.
	QueryFeedContents(filter content.Filter) []feed.Content
	// DiscoverContentIDs returns the matched content ids in all the network.
	DiscoverContentIDs(filter content.Filter) ([]string, error)
	// CheckMetadata checks the validity of the given metadata. Returns nil if valid, else an explanatory error.
	CheckMetadata(metadata content.Metadata) error
	// UpdateFeed appends the given content metadata into the peer's feed blockchain permanently. Returns the block hash.
	UpdateFeed(content.Metadata) (string, error)
	GetHashedPublicKey() [32]byte
	GetPublicKey(publicKeyHash [32]byte) *rsa.PublicKey
	GetPrivateKey() *rsa.PrivateKey
	GetUserID() string
	GetKnownUsers() map[string]struct{}
	GetFeedContents(userID string) []feed.Content
	GetReactions(contentID string) []feed.ReactionInfo
	GetUserState(userID string) feed.UserState
	BlockUser(publicKeyHash [32]byte)
	UnblockUser(publicKeyHash [32]byte)
}

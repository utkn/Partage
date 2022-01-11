package peer

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/transport"
)

type PartageClient interface {
	RegisterUser() error
	// ShareTextPost shares the text post with the given content and returns the content id, block hash.
	ShareTextPost(post content.TextPost) (string, string, error)
	// ShareCommentPost shares the comment post with the given content and reference id and returns the content id, block hash.
	ShareCommentPost(post content.CommentPost) (string, string, error)
	// DownloadPost fetches the post with the given content id from the network.
	DownloadPost(contentID string) ([]byte, error)
	// DiscoverContent returns the matched content ids.
	DiscoverContent(filter content.Filter) ([]string, error)
	// UpdateFeed appends the given content metadata into the peer's feed blockchain permanently. Returns the block hash.
	UpdateFeed(content.Metadata) (string, error)
	SharePrivatePost(msg transport.Message, recipients [][32]byte) error
	GetHashedPublicKey() [32]byte
	GetUserID() string
	GetKnownUsers() map[string]struct{}
	GetFeedContents(userID string) []content.Metadata
	GetReactions(contentID string) []content.ReactionInfo
	GetUserState(userID string) feed.UserState
	BlockUser(publicKeyHash [32]byte)
	UnblockUser(publicKeyHash [32]byte)
}

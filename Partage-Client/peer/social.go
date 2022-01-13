package peer

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/transport"
)

type SocialPeer interface {
	RegisterUser() error
	// ShareTextPost shares the text post with the given content and returns the content id, block hash.
	ShareTextPost(post content.TextPost) (content.Metadata, string, error)
	// ShareCommentPost shares the comment post with the given content and reference id and returns the content id, block hash.
	ShareCommentPost(post content.CommentPost) (content.Metadata, string, error)
	// DownloadPost fetches the post with the given content id from the network.
	DownloadPost(contentID string) ([]byte, error)
	// QueryContents queries the feed store and returns all the matching contents from the stored blockchains.
	QueryContents(filter content.Filter) []feed.Content
	// DiscoverContentIDs returns the matched content ids in all the network.
	DiscoverContentIDs(filter content.Filter) ([]string, error)
	CheckMetadata(metadata content.Metadata) error
	// UpdateFeed appends the given content metadata into the peer's feed blockchain permanently. Returns the block hash.
	UpdateFeed(content.Metadata) (string, error)
	SharePrivatePost(msg transport.Message, recipients [][32]byte) error
	GetHashedPublicKey() [32]byte
	GetUserID() string
	GetKnownUsers() map[string]struct{}
	GetFeedContents(userID string) []feed.Content
	GetReactions(contentID string) []feed.ReactionInfo
	GetUserState(userID string) feed.UserState
	BlockUser(publicKeyHash [32]byte)
	UnblockUser(publicKeyHash [32]byte)
}

package data

import "go.dedis.ch/cs438/types"

// SearchContentRequestMessage is used to search for content over the network.
type SearchContentRequestMessage struct {
	// RequestID must be a unique identifier. Use xid.New().String() to generate
	// it.
	RequestID string
	// Origin is the address of the peer that initiated the search request.
	Origin        string
	ContentFilter []byte
	Budget        uint
}

func (s SearchContentRequestMessage) NewEmpty() types.Message {
	return &SearchContentRequestMessage{}
}

func (s SearchContentRequestMessage) Name() string {
	return "searchcontentrequest"
}

func (s SearchContentRequestMessage) String() string {
	return "{searchcontentrequest}"
}

func (s SearchContentRequestMessage) HTML() string {
	return "<>"
}

type ContentInfo struct {
	ContentID string
	Metahash  string
	Chunks    [][]byte
}

// SearchContentReplyMessage describes the response of a search content request.
type SearchContentReplyMessage struct {
	// RequestID must be the same as the RequestID set in the
	// SearchContentRequestMessage.
	RequestID string
	Responses []ContentInfo
}

func (s SearchContentReplyMessage) NewEmpty() types.Message {
	return &SearchContentReplyMessage{}
}

func (s SearchContentReplyMessage) Name() string {
	return "searchcontentreply"
}

func (s SearchContentReplyMessage) String() string {
	return "{searchcontentreply}"
}

func (s SearchContentReplyMessage) HTML() string {
	return "<>"
}

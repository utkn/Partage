package types

import (
	"go.dedis.ch/cs438/transport"
)

//----------CA communication
type CertificateAuthorityMessage struct {
	Type    string
	Payload []byte
}
type Registration struct{
	SignedCertificate []byte
	PublicKeySignature []byte
}

//======================PARTAGE
//---------------Post
type Post struct {
	//Sign(Hash(Content+Timestamp)) ..provides integrity and authenticity to Post
	Check *transport.Validation 

	Timestamp int64

	//only allow Content with a maximum of 128 characters (bytes)
	Content string
}

//---------------Private Post
type RecipientsMap map[[32]byte][128]byte
type PrivatePost struct {
	// Recipients is a bag of recipients that maps to encrypted symmetric-key
	Recipients []byte// map[[32]byte][32]byte --> RecipientsMap

	// Msg is the private message to be read by the recipients
	Msg []byte //encrypted transport.Message with AES-256 ---> NOTE: this should be a Post message!
}

//----------------Search for Signed Public Key in the network
type SearchPKRequestMessage struct {
	// RequestID must be a unique identifier. Use xid.New().String() to generate it.
	RequestID string
	// Origin is the address of the peer that initiated the search request.
	Origin string

	Username [32]byte
	Budget   uint
}

// SearchPKReplyMessage describes the response of a search PK request.
type SearchPKReplyMessage struct {
	// RequestID must be the same as the RequestID set in the SearchPKRequestMessage.
	//Response  []byte //encoded types.SignedPublicKey
	Response transport.SignedPublicKey
	RequestID string
}

/*
//---------------SignedMessage
type SignedMessage struct{
	SrcPublicKey transport.SignedPublicKey
	Signature []byte
	Msg *transport.Message
} 
*/
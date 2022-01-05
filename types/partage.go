package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

// -----------------------------------------------------------------------------
// Post

// NewEmpty implements types.Message.
func (p Post) NewEmpty() Message {
	return &Post{}
}

// Name implements types.Message.
func (p Post) Name() string {
	return "post"
}

// String implements types.Message.
func (p Post) String() string {
	return fmt.Sprintf("post{%s} at {%d}", p.Content,p.Timestamp)
}

// HTML implements types.Message.
func (p Post) HTML() string {
	return fmt.Sprintf("post{%s}", p.Content)
}

// -----------------------------------------------------------------------------
// PrivatePost

// NewEmpty implements types.Message.
func (p PrivatePost) NewEmpty() Message {
	return &PrivatePost{}
}

// Name implements types.Message.
func (p PrivatePost) Name() string {
	return "privatePost"
}

// String implements types.Message.
func (p PrivatePost) String() string {
	return fmt.Sprintf("private post for %s", p.Recipients)
}

// HTML implements types.Message.
func (p PrivatePost) HTML() string {
	return fmt.Sprintf("private post for %s", p.Recipients)
}

// -----------------------------------------------------------------------------
// SearchPKRequestMessage

// NewEmpty implements types.Message.
func (s SearchPKRequestMessage) NewEmpty() Message {
	return &SearchPKRequestMessage{}
}

// Name implements types.Message.
func (s SearchPKRequestMessage) Name() string {
	return "searchpkrequest"
}

// String implements types.Message.
func (s SearchPKRequestMessage) String() string {
	return fmt.Sprintf("searchpkrequest{%s %d}", s.Username, s.Budget)
}

// HTML implements types.Message.
func (s SearchPKRequestMessage) HTML() string {
	return s.String()
}

// -----------------------------------------------------------------------------
// SearchPKReplyMessage

// NewEmpty implements types.Message.
func (s SearchPKReplyMessage) NewEmpty() Message {
	return &SearchPKReplyMessage{}
}

// Name implements types.Message.
func (s SearchPKReplyMessage) Name() string {
	return "searchpkreply"
}

// String implements types.Message.
func (s SearchPKReplyMessage) String() string {
	return fmt.Sprintf("searchpkreply{%s}", s.RequestID)
}

// HTML implements types.Message.
func (s SearchPKReplyMessage) HTML() string {
	return fmt.Sprintf("searchpkreply %s", s.RequestID)
}

// -----------------------------------------------------------------------------
// CertificateAuthorityMessage
func (m *CertificateAuthorityMessage) Decode(bytes []byte) error{
	return json.Unmarshal(bytes,&m)
}
// Registration that comes with the CertificateAuthorityMessage
func (r *Registration) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &r)
}



// RecipientsMap map[hash(rsaPublicKeyA)]EncPKA(AESKey) == map[[32]byte][128]byte
func (r RecipientsMap) Encode() ([]byte,error){
	window:=32+128
	encoded:=make([]byte,len(r)*(window))
	i:=0
	for k,v:=range r{
		if len(k)!=32 || len(v)!=128{
			return nil,errors.New("invalid RecipientsMap")
		}
		copy(encoded[i:i+window],append(k[:],v[:]...))
		i+=window
	}
	return encoded,nil
}

func (r RecipientsMap) Decode(encoded []byte) (error){
	windowK:=32
	windowV:=128
	window:=windowK+windowV
	if len(encoded)%window!=0{
		return errors.New("invalid encoded RecipientsMap")
	}
	var k [32]byte
	var v [128]byte
	for i:=0;i<=len(encoded)-window;i+=window{
		copy(k[:],encoded[i:i+windowK])
		copy(v[:],encoded[i+windowK:i+window])
		r[k]=v
	}
	return nil
}
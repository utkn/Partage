package types

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"go.dedis.ch/cs438/transport"
)

//--------------------------------------------------------------
// Rumor
func (r *Rumor) AddValidation(myPrivateKey *rsa.PrivateKey, mySignedPublicKey *transport.SignedPublicKey) error{
	//Adds a validation check to Rumor (provides integrity and authenticity check!)
	byteMsg, err := json.Marshal(r.Msg)
	if err!=nil{
		return err
	}
	// Hash(rumor.Msg||rumor.Origin||rumor.Sequence)
	hashedContent:=transport.Hash(append(byteMsg,append([]byte(r.Origin),[]byte(strconv.Itoa(int(r.Sequence)))...)...))
	signature, err := rsa.SignPKCS1v15(rand.Reader, myPrivateKey, crypto.SHA256, hashedContent[:])
	if err != nil {
		return err
	}

	r.Check =&transport.Validation{
		Signature: signature,
		SrcPublicKey: *mySignedPublicKey,
	}

	return nil
}

func (r *Rumor) Validate(publicKeyCA *rsa.PublicKey) error{
	if r.Check==nil{
		return fmt.Errorf("rumor has empty validation check")
	}

	//1- Check if the Public Key of the user who signed the message is valid (signed by trusted CA)
	srcPKBytes, err := x509.MarshalPKIXPublicKey(r.Check.SrcPublicKey.PublicKey)
	if err != nil {
		return err
	}
	hashedSrcPK := transport.Hash(srcPKBytes)
	if err:=rsa.VerifyPKCS1v15(publicKeyCA,crypto.SHA256,hashedSrcPK[:],r.Check.SrcPublicKey.Signature); err!=nil{
		//invalid SignedPublicKey (not signed by trusted CA)
		return fmt.Errorf("rumor's src public key is not signed by trusted CA")
	}

	//2- Check if Hash(rumor.Msg||rumor.Origin||rumor.Sequence) was signed by src's private key
	byteMsg,_:=json.Marshal(r.Msg)
	hashedContent :=transport.Hash(append(byteMsg,append([]byte(r.Origin),[]byte(strconv.FormatInt(int64(r.Sequence), 10))...)...))
	if rsa.VerifyPKCS1v15(r.Check.SrcPublicKey.PublicKey,crypto.SHA256,hashedContent[:],r.Check.Signature)!=nil{
		//invalid signature
		return fmt.Errorf("rumor is not signed by src private key")
	}

	//VALID!
	return nil
}

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
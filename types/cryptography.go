package types

import (
	"encoding/json"
	"errors"

)

// -----------------------------------------------------------------------------
// CertificateAuthorityMessage
func (m *CertificateAuthorityMessage) Decode(bytes []byte) error{
	return json.Unmarshal(bytes,&m)
}
// Registration that comes with the CertificateAuthorityMessage
func (r *Registration) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &r)
}
func (k *SignedPublicKey) Encode() ([]byte, error) {
	return json.Marshal(k)
}

func (k *SignedPublicKey) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &k)
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
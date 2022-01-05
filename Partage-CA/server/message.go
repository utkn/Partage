package server

import "encoding/json"

type Message struct {
	Type    string
	Payload []byte
}

func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &m)
}

//--------
type Registration struct{
	SignedCertificate []byte
	PublicKeySignature []byte
}

func (r *Registration) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Registration) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &r)
}
package types

import "crypto/rsa"

//----------CA communication
type CertificateAuthorityMessage struct {
	Type    string
	Payload []byte
}
type Registration struct{
	SignedCertificate []byte
	PublicKeySignature []byte
}
//---------
type SignedPublicKey struct{
	PublicKey *rsa.PublicKey
	Signature []byte
}

type RecipientsMap map[[32]byte][128]byte
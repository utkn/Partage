package cryptography

import (
	"crypto/ecdsa"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
)

type Layer struct {
	network *network.Layer
	
	// ...
	config *peer.Configuration
	privateKey *ecdsa.PrivateKey 


}

func Construct(network *network.Layer, config *peer.Configuration) *Layer {
	privKey,_:=utils.GenerateKeyPair() //NOTE: should be the same key pair used in the tls certificate, not a newly generated one

	return &Layer{
		network:                  network,
		config: config,
		privateKey: privKey,
		// ...
	}
}

func (l *Layer) GetAddress() string {
	return l.network.GetAddress()
}

func (l *Layer) GetNetwork() *network.Layer { //NOTE: temporary
	return l.network
}

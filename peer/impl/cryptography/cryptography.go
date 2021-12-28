package cryptography

import (
	"crypto/rsa"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/tcptls"
)

type Layer struct {
	network *network.Layer

	// ...
	config     *peer.Configuration
	privateKey *rsa.PrivateKey
}

func Construct(network *network.Layer, config *peer.Configuration) *Layer {
	socket, ok := config.Socket.(*tcptls.Socket)
	if !ok {
		panic("node must have a tcp socket in order to use tls")
	}
	return &Layer{
		network:    network,
		config:     config,
		privateKey: socket.GetCertificate().PrivateKey.(*rsa.PrivateKey),
		// ...
	}
}

func (l *Layer) GetAddress() string {
	return l.network.GetAddress()
}

func (l *Layer) GetNetwork() *network.Layer { //NOTE: temporary
	return l.network
}

func (l *Layer) SendPrivatePost(msg transport.Message, recipients []string) error {
	//TODO: ...
}

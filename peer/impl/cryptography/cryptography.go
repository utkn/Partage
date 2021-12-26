package cryptography

import (
	"go.dedis.ch/cs438/peer/impl/network"
)

type Layer struct {
	network *network.Layer
	// ...

}

func Construct(network *network.Layer) *Layer {

	return &Layer{
		network:                  network,
		
		// ...
	}
}

func (l *Layer) GetAddress() string {
	return l.network.GetAddress()
}

func (l *Layer) GetNetwork() *network.Layer { //NOTE: temporary
	return l.network
}

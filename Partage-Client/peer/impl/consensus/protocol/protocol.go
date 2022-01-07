package protocol

import "go.dedis.ch/cs438/peer"

// Protocol represents a consensus protocol.
type Protocol interface {
	Propose(interface{}) error
	RegisterHandlers(*peer.Configuration) error
}

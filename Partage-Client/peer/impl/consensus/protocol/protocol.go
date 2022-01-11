package protocol

import (
	"go.dedis.ch/cs438/types"
)

// Protocol represents a consensus protocol.
type Protocol interface {
	Propose(types.PaxosValue) (string, error)
	GetProtocolID() string
	HandleConsensusMessage(ConsensusMessage) error
}

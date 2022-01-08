package protocol

import (
	"go.dedis.ch/cs438/types"
)

// Protocol represents a consensus protocol.
type Protocol interface {
	Propose(types.PaxosValue) error
	HandleConsensusMessage(message ConsensusMessage) error
}

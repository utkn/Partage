package protocol

import (
	"go.dedis.ch/cs438/registry"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// ConsensusMessage represents a message that should be handled by the consensus layer.
type ConsensusMessage struct {
	ProtocolID string
	InnerMsg   transport.Message
}

func (c ConsensusMessage) NewEmpty() types.Message {
	return &ConsensusMessage{}
}

func (c ConsensusMessage) Name() string {
	return "consensus"
}

func (c ConsensusMessage) String() string {
	return "{consensus ...}"
}

func (c ConsensusMessage) HTML() string {
	return "<>"
}

func WrapInConsensusMessage(protocolID string, tMsg transport.Message) ConsensusMessage {
	return ConsensusMessage{
		ProtocolID: protocolID,
		InnerMsg:   tMsg,
	}
}

func UnwrapConsensusMessage(consensusMsg ConsensusMessage) types.Message {
	innerMsg, _ := registry.GlobalRegistry.GetMessage(&consensusMsg.InnerMsg)
	return innerMsg
}

package protocol

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/registry"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

type ConsensusMessage struct {
	InnerMsg transport.Message
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
	return "<CONSENSUS MSG HTML>"
}

func WrapInConsensusPacket(config *peer.Configuration, tMsg transport.Message) transport.Message {
	msg := ConsensusMessage{
		InnerMsg: tMsg,
	}
	t, _ := config.MessageRegistry.MarshalMessage(&msg)
	return t
}

func UnwrapConsensusMessage(consensusMsg ConsensusMessage) types.Message {
	innerMsg, _ := registry.GlobalRegistry.GetMessage(&consensusMsg.InnerMsg)
	return innerMsg
}

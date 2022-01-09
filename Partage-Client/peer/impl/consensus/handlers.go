package consensus

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(protocol.ConsensusMessage{}, l.HandleConsensusMessage)
	// The following handlers are registered for backwards compatibility.
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosAcceptMessage{}, l.HandlePaxosProxy)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosProposeMessage{}, l.HandlePaxosProxy)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosPrepareMessage{}, l.HandlePaxosProxy)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosPromiseMessage{}, l.HandlePaxosProxy)
	l.Config.MessageRegistry.RegisterMessageCallback(types.TLCMessage{}, l.HandlePaxosProxy)
}

// HandleConsensusMessage forwards a consensus message to registered protocols.
func (l *Layer) HandleConsensusMessage(msg types.Message, pkt transport.Packet) error {
	consensusMsg, ok := msg.(*protocol.ConsensusMessage)
	if !ok {
		return fmt.Errorf("could not parse the received consensus msg")
	}
	// Route the consensus message to the appropriate protocol. Let them decide what to do with it.
	if consensusMsg.InnerMsg.Type == "paxosaccept" {
		println(l.GetAddress(), "HAS RECEIVED AN ACCEPT FROM", pkt.Header.Source)
	}
	p, ok := l.protocols[consensusMsg.ProtocolID]
	if !ok {
		return fmt.Errorf("consensus layer could not find a protocol with id %s", consensusMsg.ProtocolID)
	}
	return p.HandleConsensusMessage(*consensusMsg)
}

// HandlePaxosProxy listens to usual paxos messages, wraps them into consensus messages and processes them locally.
func (l *Layer) HandlePaxosProxy(msg types.Message, pkt transport.Packet) error {
	wrappedMsg := protocol.WrapInConsensusMessage("default", *pkt.Msg)
	wrappedTransportMsg, _ := l.Config.MessageRegistry.MarshalMessage(wrappedMsg)
	wrappedPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    &wrappedTransportMsg,
	}
	return l.Config.MessageRegistry.ProcessPacket(wrappedPkt)
}

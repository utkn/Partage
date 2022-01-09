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
	l.RLock()
	defer l.RUnlock()
	consensusMsg, ok := msg.(*protocol.ConsensusMessage)
	if !ok {
		return fmt.Errorf("could not parse the received consensus msg")
	}
	// Route the consensus message to the appropriate protocols. Let them decide what to do with it.
	for _, p := range l.protocols {
		// Skip the unrelated protocols.
		if p.GetProtocolID() != consensusMsg.ProtocolID {
			continue
		}
		err := p.HandleConsensusMessage(*consensusMsg)
		if err != nil {
			return err
		}
	}
	return nil
}

// HandlePaxosProxy listens to usual paxos messages, wraps them into consensus messages and processes them locally.
func (l *Layer) HandlePaxosProxy(msg types.Message, pkt transport.Packet) error {
	wrappedMsg := protocol.WrapInConsensusPacket("default", l.Config, *pkt.Msg)
	wrappedPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    &wrappedMsg,
	}
	return l.Config.MessageRegistry.ProcessPacket(wrappedPkt)
}

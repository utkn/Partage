package consensus

import (
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosAcceptMessage{}, l.PaxosAcceptMessageHandler)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosPromiseMessage{}, l.PaxosPromiseMessageHandler)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosPrepareMessage{}, l.PaxosPrepareMessageHandler)
	l.Config.MessageRegistry.RegisterMessageCallback(types.PaxosProposeMessage{}, l.PaxosProposeMessageHandler)
	l.Config.MessageRegistry.RegisterMessageCallback(types.TLCMessage{}, l.TLCMessageHandler)
}

func (l *Layer) PaxosPrepareMessageHandler(msg types.Message, pkt transport.Packet) error {
	prepareMsg := msg.(*types.PaxosPrepareMessage)
	return l.acceptor.HandlePrepare(*prepareMsg)
}

func (l *Layer) PaxosProposeMessageHandler(msg types.Message, pkt transport.Packet) error {
	proposeMsg := msg.(*types.PaxosProposeMessage)
	return l.acceptor.HandlePropose(*proposeMsg)
}

func (l *Layer) TLCMessageHandler(msg types.Message, pkt transport.Packet) error {
	tlcMsg := msg.(*types.TLCMessage)
	return l.acceptor.HandleTLC(*tlcMsg)
}

func (l *Layer) PaxosPromiseMessageHandler(msg types.Message, pkt transport.Packet) error {
	_ = l.Proposer.Accept(msg)
	return nil
}

func (l *Layer) PaxosAcceptMessageHandler(msg types.Message, pkt transport.Packet) error {
	_ = l.Proposer.Accept(msg)
	acceptMsg := msg.(*types.PaxosAcceptMessage)
	return l.acceptor.HandleAccept(*acceptMsg)
}

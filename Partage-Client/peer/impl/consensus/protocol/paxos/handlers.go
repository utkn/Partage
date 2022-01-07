package paxos

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (p *Paxos) RegisterHandlers(config *peer.Configuration) error {
	config.MessageRegistry.RegisterMessageCallback(types.PaxosAcceptMessage{}, p.PaxosAcceptMessageHandler)
	config.MessageRegistry.RegisterMessageCallback(types.PaxosPromiseMessage{}, p.PaxosPromiseMessageHandler)
	config.MessageRegistry.RegisterMessageCallback(types.PaxosPrepareMessage{}, p.PaxosPrepareMessageHandler)
	config.MessageRegistry.RegisterMessageCallback(types.PaxosProposeMessage{}, p.PaxosProposeMessageHandler)
	config.MessageRegistry.RegisterMessageCallback(types.TLCMessage{}, p.TLCMessageHandler)
	return nil
}

func (p *Paxos) PaxosPrepareMessageHandler(msg types.Message, pkt transport.Packet) error {
	prepareMsg := msg.(*types.PaxosPrepareMessage)
	return p.acceptor.HandlePrepare(*prepareMsg)
}

func (p *Paxos) PaxosProposeMessageHandler(msg types.Message, pkt transport.Packet) error {
	proposeMsg := msg.(*types.PaxosProposeMessage)
	return p.acceptor.HandlePropose(*proposeMsg)
}

func (p *Paxos) TLCMessageHandler(msg types.Message, pkt transport.Packet) error {
	tlcMsg := msg.(*types.TLCMessage)
	return p.acceptor.HandleTLC(*tlcMsg)
}

func (p *Paxos) PaxosPromiseMessageHandler(msg types.Message, pkt transport.Packet) error {
	_ = p.Proposer.Accept(msg)
	return nil
}

func (p *Paxos) PaxosAcceptMessageHandler(msg types.Message, pkt transport.Packet) error {
	_ = p.Proposer.Accept(msg)
	acceptMsg := msg.(*types.PaxosAcceptMessage)
	return p.acceptor.HandleAccept(*acceptMsg)
}

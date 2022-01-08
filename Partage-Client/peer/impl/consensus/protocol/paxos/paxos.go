package paxos

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type Paxos struct {
	protocol.Protocol
	Clock          *Clock
	acceptor       *Acceptor
	Proposer       *StateMachine
	LastProposalID uint

	Notification *utils.AsyncNotificationHandler
	Gossip       *gossip.Layer
	Config       *peer.Configuration
}

func New(config *peer.Configuration, gossip *gossip.Layer, blockFactory BlockFactory) *Paxos {
	p := Paxos{
		Clock:          NewClock(),
		Proposer:       &StateMachine{},
		LastProposalID: config.PaxosID,

		Notification: utils.NewAsyncNotificationHandler(),
		Gossip:       gossip,
		Config:       config,
	}
	// Create the acceptor. Acceptor methods will be invoked by the message handlers.
	p.acceptor = &Acceptor{
		paxos:        &p,
		blockFactory: blockFactory,
	}
	return &p
}

func (p *Paxos) Propose(val types.PaxosValue) error {
	p.Proposer.Run(ProposerBeginState{
		paxos: p,
		value: val,
	})
	return nil
}

func (p *Paxos) HandleConsensusMessage(msg protocol.ConsensusMessage) error {
	innerMsg := protocol.UnwrapConsensusMessage(msg)
	switch innerMsg.Name() {
	case "paxosaccept":
		p.Proposer.Input(innerMsg)
		acceptMsg := innerMsg.(*types.PaxosAcceptMessage)
		return p.acceptor.HandleAccept(*acceptMsg)
	case "paxospromise":
		p.Proposer.Input(innerMsg)
		return nil
	case "paxosprepare":
		prepareMsg := innerMsg.(*types.PaxosPrepareMessage)
		return p.acceptor.HandlePrepare(*prepareMsg)
	case "paxospropose":
		proposeMsg := innerMsg.(*types.PaxosProposeMessage)
		return p.acceptor.HandlePropose(*proposeMsg)
	case "tlc":
		tlcMsg := innerMsg.(*types.TLCMessage)
		return p.acceptor.HandleTLC(*tlcMsg)
	}
	return nil
}

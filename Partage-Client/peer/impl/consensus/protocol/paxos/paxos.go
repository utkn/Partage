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

func New(config *peer.Configuration, gossip *gossip.Layer) *Paxos {
	p := Paxos{
		Clock:          NewClock(),
		Proposer:       &StateMachine{},
		LastProposalID: config.PaxosID,

		Notification: utils.NewAsyncNotificationHandler(),
		Gossip:       gossip,
		Config:       config,
	}
	// Create the acceptor.
	p.acceptor = &Acceptor{
		paxos: &p,
	}
	return &p
}

func (p *Paxos) Propose(val interface{}) error {
	value := val.(types.PaxosValue)
	p.Proposer.Run(ProposerBeginState{
		paxos: p,
		value: value,
	})
	return nil
}

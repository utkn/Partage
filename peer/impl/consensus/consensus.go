package consensus

import (
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	Gossip         *gossip.Layer
	Config         *peer.Configuration
	Notification   *utils.AsyncNotificationHandler
	Clock          *Clock
	acceptor       *Acceptor
	Proposer       *StateMachine
	LastProposalID uint
}

func Construct(gossip *gossip.Layer, config *peer.Configuration) *Layer {
	layer := &Layer{
		Gossip:         gossip,
		Config:         config,
		Notification:   utils.NewAsyncNotificationHandler(),
		Clock:          NewClock(),
		LastProposalID: config.PaxosID,
		Proposer:       &StateMachine{},
	}
	layer.acceptor = &Acceptor{
		clock:        layer.Clock,
		config:       layer.Config,
		gossip:       layer.Gossip,
		notification: layer.Notification,
	}
	return layer
}

func (l *Layer) GetAddress() string {
	return l.Gossip.GetAddress()
}

func (l *Layer) Propose(value types.PaxosValue) error {
	// Consensus should not be invoked when there are <= 1 many peers.
	if l.Config.TotalPeers <= 1 {
		return fmt.Errorf("consensus is disabled for <= 1 many peers")
	}
	// Initiate the Paxos consensus protocol.
	l.Proposer.Run(ProposerBeginState{
		consensus: l,
		value:     value,
	})
	//// Check this for the cases where we may not have captured the completion of the proposal.
	//if l.Config.Storage.GetNamingStore().Get(value.Filename) != nil {
	//	break
	//}
	return nil
}

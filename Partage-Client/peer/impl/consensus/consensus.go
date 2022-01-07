package consensus

import (
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	Gossip *gossip.Layer
	Config *peer.Configuration

	protocols map[string]protocol.Protocol
}

func Construct(gossip *gossip.Layer, config *peer.Configuration) *Layer {
	layer := &Layer{
		Gossip:    gossip,
		Config:    config,
		protocols: make(map[string]protocol.Protocol),
	}
	// As the default protocol, use Paxos.
	layer.RegisterProtocol("default", paxos.New(config, gossip))
	return layer
}

func (l *Layer) RegisterProtocol(id string, protocol protocol.Protocol) {
	l.protocols[id] = protocol
	protocol.RegisterHandlers(l.Config)
}

func (l *Layer) Propose(value types.PaxosValue) error {
	return l.ProposeWithProtocol("default", value)
}

func (l *Layer) ProposeWithProtocol(protocolID string, value interface{}) error {
	// Consensus should not be invoked when there are <= 1 many peers.
	if l.Config.TotalPeers <= 1 {
		return fmt.Errorf("consensus is disabled for <= 1 many peers")
	}
	// Initiate the Paxos consensus protocol.
	l.protocols[protocolID].Propose(value)
	//// Check this for the cases where we may not have captured the completion of the proposal.
	//if l.Config.Storage.GetNamingStore().Get(value.Filename) != nil {
	//	break
	//}
	return nil
}

func (l *Layer) RegisterHandlers() {
	// Handled by each registered protocol separately.
}

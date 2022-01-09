package gossip

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/cryptography"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Layer struct {
	network      *network.Layer
	cryptography *cryptography.Layer

	config          *peer.Configuration
	rumorLock       sync.Mutex
	view            *PeerView
	ackNotification *utils.AsyncNotificationHandler
	quitDistributor *utils.SignalDistributor
}

func Construct(network *network.Layer, cryptography *cryptography.Layer, config *peer.Configuration, quitDistributor *utils.SignalDistributor) *Layer {
	layer := &Layer{
		network:         network,
		cryptography:    cryptography,
		config:          config,
		view:            NewPeerView(),
		ackNotification: utils.NewAsyncNotificationHandler(),
		quitDistributor: quitDistributor,
	}
	// Register the quit listeners.
	quitDistributor.NewListener("antientropy")
	quitDistributor.NewListener("heartbeat")
	// Initiate the anti entropy mechanism.
	if config.AntiEntropyInterval > 0 {
		go AntiEntropy(layer, config.AntiEntropyInterval)
	}
	// Initiate the heartbeat mechanism.
	if config.HeartbeatInterval > 0 {
		go Heartbeat(layer, config.HeartbeatInterval)
	}
	return layer
}

func (l *Layer) GetAddress() string {
	return l.network.GetAddress()
}

// BroadcastMessage broadcasts a given message to the network using rumors. The message will also be handled locally.
func (l *Layer) BroadcastMessage(msg types.Message) error {
	utils.PrintDebug("gossip", l.GetAddress(), "is broadcasting", msg.Name())
	tMsg, _ := l.config.MessageRegistry.MarshalMessage(msg)
	return l.Broadcast(tMsg)
}

func (l *Layer) GetViewAsStatusMsg() types.StatusMessage {
	return l.view.AsStatusMsg()
}

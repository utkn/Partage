package gossip

import (
	"fmt"
	"sync"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/cryptography"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	network         *network.Layer
	cryptography    *cryptography.Layer
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

func (l *Layer) SendRumorsMsg(msg transport.Message, unresponsiveNeighbors map[string]struct{}) error {
	// Prepare the message to be sent to a random neighbor.
	randNeighbor, err := l.network.ChooseRandomNeighbor(unresponsiveNeighbors) //TODO:
	// If we could not find a random neighbor, terminate broadcast.
	if err != nil {
		utils.PrintDebug("communication", l.GetAddress(), "is terminating random unicast as there are no possible neighbors.")
		return nil
	}
	// Create a header for the rumors message.
	header := transport.NewHeader(l.GetAddress(), l.GetAddress(), randNeighbor, 0)
	pkt := transport.Packet{
		Header: &header,
		Msg:    &msg,
	}
	// Then, send it to the random peer selected without using the routing table.
	utils.PrintDebug("network", l.GetAddress(), "is sending", randNeighbor, "a", pkt.Msg.Type)
	if l.cryptography != nil {
		//send it via the cryptography layer (signed header)
		err = l.cryptography.Send(randNeighbor, pkt.Copy(), time.Second*5)
		if err != nil {
			return fmt.Errorf("could not unicast the rumors message, using the crypto layer, within the broadcast: %w", err)
		}
	} else {
		err = l.network.Send(randNeighbor, pkt.Copy(), time.Second*1)
		if err != nil {
			return fmt.Errorf("could not unicast the rumors message within the broadcast: %w", err)
		}
	}
	// Wait for an Ack.
	if l.config.AckTimeout > 0 {
		utils.PrintDebug("gossip", l.GetAddress(), "is now waiting for an acknowledgement with packet id", pkt.Header.PacketID)
		ack := l.ackNotification.ResponseCollector(pkt.Header.PacketID, l.config.AckTimeout)
		if ack == nil {
			utils.PrintDebug("gossip", l.GetAddress(), "has waited long enough for an ack!")
			unresponsiveNeighbors[randNeighbor] = struct{}{}
			return l.SendRumorsMsg(msg, unresponsiveNeighbors)
		} else {
			utils.PrintDebug("gossip", l.GetAddress(), "has received the ack!")
		}
	}
	return nil
}

func (l *Layer) Broadcast(msg transport.Message) error {
	utils.PrintDebug("communication", l.GetAddress(), "is broadcasting", msg.Type)
	// First, locally process the message.
	localHeader := transport.NewHeader(l.GetAddress(), l.GetAddress(), l.GetAddress(), 0)
	localPkt := transport.Packet{
		Header: &localHeader,
		Msg:    &msg,
	}
	// Locally handle the message in the background -- simulate a message receipt.
	go func() {
		utils.PrintDebug("communication", l.GetAddress(), "is locally handling", msg.Type, " - START")
		err := l.config.MessageRegistry.ProcessPacket(localPkt.Copy())
		utils.PrintDebug("communication", l.GetAddress(), "has locally handled", msg.Type, " - END")
		if err != nil {
			fmt.Printf("could not process broadcast packet %s locally: %s", msg.Type, err)
		}
	}()
	return l.BroadcastAway(msg)
}

// BroadcastAway broadcasts the given message to the network using rumors.
func (l *Layer) BroadcastAway(msg transport.Message) error {
	// Wrap the message in a rumor.
	rumor := types.Rumor{}
	rumor.Msg = &msg
	rumor.Origin = l.GetAddress()
	rumor.Sequence = l.view.GetSequence(l.GetAddress()) + 1
	if l.cryptography != nil {
		if err := rumor.AddValidation(l.cryptography.GetPrivateKey(), l.cryptography.GetSignedPublicKey()); err != nil {
			fmt.Println("broadcast away:", err)
			return err
		}
	}
	// Wrap the rumor in a rumors message.
	rumorsMsg := types.RumorsMessage{}
	rumorsMsg.Rumors = append(rumorsMsg.Rumors, rumor)
	// Convert the rumors message into a transport message.
	rumorsTranspMsg, err := l.config.MessageRegistry.MarshalMessage(&rumorsMsg)
	if err != nil {
		return fmt.Errorf("could not marshal rumors message into a transport message: %w", err)
	}
	l.view.SaveRumor(rumor)
	utils.PrintDebug("communication", l.GetAddress(), "is sending a rumors msg with", msg.Type)
	return l.SendRumorsMsg(rumorsTranspMsg, make(map[string]struct{}))
}

func (l *Layer) GetViewAsStatusMsg() types.StatusMessage {
	return l.view.AsStatusMsg()
}

package gossip

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"time"
)

// Broadcast broadcasts the given transport message to the network using rumors. The message will also be handled locally.
// Kept as public for backward compatibility.
func (l *Layer) Broadcast(msg transport.Message) error {
	// First, locally process the message.
	localHeader := transport.NewHeader(l.GetAddress(), l.GetAddress(), l.GetAddress(), 0)
	localPkt := transport.Packet{
		Header: &localHeader,
		Msg:    &msg,
	}
	// Locally handle the message in the background, simulating a message receipt.
	go func() {
		err := l.config.MessageRegistry.ProcessPacket(localPkt.Copy())
		if err != nil {
			fmt.Printf("could not process broadcast packet %s locally: %s", msg.Type, err)
		}
	}()
	return l.broadcastAway(msg)
}

// broadcastAway broadcasts the given message to the network using rumors. The message won't be handled locally.
func (l *Layer) broadcastAway(msg transport.Message) error {
	// Wrap the message in a rumor.
	rumor := types.Rumor{}
	rumor.Msg = &msg
	rumor.Origin = l.GetAddress()
	// Beginning of critical section (atomic update of sequence numbers).
	l.rumorLock.Lock()
	rumor.Sequence = uint(l.view.GetSequence(l.GetAddress()) + 1)
	if l.cryptography != nil {
		if err := rumor.AddValidation(l.cryptography.GetPrivateKey(), l.cryptography.GetSignedPublicKey()); err != nil {
			fmt.Println("broadcast away:", err)
			return err
		}
	}
	l.view.SaveRumor(rumor)
	// End of critical section.
	l.rumorLock.Unlock()
	// Wrap the rumor in a rumors message.
	rumorsMsg := types.RumorsMessage{}
	rumorsMsg.Rumors = append(rumorsMsg.Rumors, rumor)
	utils.PrintDebug("communication", l.GetAddress(), "is sending a rumors msg with", msg.Type)
	return l.sendRumors(rumorsMsg, make(map[string]struct{}))
}

func (l *Layer) sendRumors(msg types.RumorsMessage, unresponsiveNeighbors map[string]struct{}) error {
	rumorsTranspMsg, err := l.config.MessageRegistry.MarshalMessage(&msg)
	if err != nil {
		return fmt.Errorf("could not marshal rumors message into a transport message: %w", err)
	}
	// Prepare the message to be sent to a random neighbor.
	randNeighbor, err := l.network.ChooseRandomNeighbor(unresponsiveNeighbors)
	// If we could not find a random neighbor, terminate broadcast.
	if err != nil {
		utils.PrintDebug("communication", l.GetAddress(), "is terminating random unicast as there are no possible neighbors.")
		return nil
	}
	// Create a header for the rumors message.
	header := transport.NewHeader(l.GetAddress(), l.GetAddress(), randNeighbor, 0)
	pkt := transport.Packet{
		Header: &header,
		Msg:    &rumorsTranspMsg,
	}
	// Then, send it to the random peer selected without using the routing table.
	utils.PrintDebug("network", l.GetAddress(), "is sending", randNeighbor, "a rumor with embedded", msg.Rumors[0].Msg.Type)
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
			return l.sendRumors(msg, unresponsiveNeighbors)
		} else {
			utils.PrintDebug("gossip", l.GetAddress(), "has received the ack!")
		}
	}
	return nil
}

package gossip

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"math/rand"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(types.RumorsMessage{}, l.RumorsMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.StatusMessage{}, l.StatusMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.AckMessage{}, l.AckMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.PrivatePost{}, l.PrivatePostHandler) //Partage
	l.config.MessageRegistry.RegisterMessageCallback(types.Post{}, l.PostHandler)               //Partage
}

func (l *Layer) RumorsMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("gossip", l.GetAddress(), "is at RumorsMessageHandler")
	rumorsMsg, ok := msg.(*types.RumorsMessage)
	if !ok {
		return fmt.Errorf("could not parse rumors message")
	}
	// Find the rumors of interest (i.e., expected rumors)
	var rumorsOfInterest []types.Rumor
	// Concurrent executions of this procedure will cause problems. We mark this as a critical section.
	l.rumorLock.Lock()
	// Go through the received rumors and identify the ones that should be handled. Also add them to the view, updating
	// the sequence number.
	for _, rumor := range rumorsMsg.Rumors {
		if rumor.Msg.Type == "consensus" {
			var c protocol.ConsensusMessage
			l.config.MessageRegistry.UnmarshalMessage(rumor.Msg, &c)
		}
		// Only consider the expected rumors.
		if l.view.IsExpected(rumor.Origin, int64(rumor.Sequence)) {
			// Validate rumor's signature
			if l.cryptography != nil {
				if err := rumor.Validate(l.cryptography.GetCAPublicKey()); err != nil {
					fmt.Println("dropped rumor due to invalid signature..", err)
					continue
				} else {
					// Valid..
					bytesPK, _ := utils.PublicKeyToBytes(rumor.Check.SrcPublicKey.PublicKey)
					hashPK := utils.Hash(bytesPK)

					//check if rumor is from blocked user
					if l.cryptography.IsBlocked(hashPK) {
						if !l.cryptography.IsBlockedIP(rumor.Origin) {
							//store blocked user ip adr
							l.cryptography.AddBlockedIP(rumor.Origin, hashPK)
							//drop view
							l.view.DropViewFrom(rumor.Origin)
						}
						fmt.Println("just ignored rumor from blocked user!")

						continue
					}

					//Store rumor's SignedPublicKey in Catalog..(helps to get to know users in the network!)
					if _, exists := l.cryptography.GetUserFromCatalog(hashPK); !exists {
						l.cryptography.AddUserToCatalog(hashPK, &rumor.Check.SrcPublicKey)
					}
				}
			}
			utils.PrintDebug("gossip", l.GetAddress(), "will process a rumor from", rumor.Origin, "with sequence", rumor.Sequence)
			rumorsOfInterest = append(rumorsOfInterest, rumor)
			// Save the rumor.
			l.view.SaveRumor(rumor)
			// Update the routing table with the rumor origin.
			l.network.SetRoutingEntry(rumor.Origin, pkt.Header.RelayedBy)
		}
	}
	// End of critical section.
	l.rumorLock.Unlock()
	// Process the messages contained within the rumors of interest.
	for _, rumor := range rumorsOfInterest {
		// Create a new packet to process locally.
		newPkt := transport.Packet{
			Header: pkt.Header,
			Msg:    rumor.Msg,
		}
		// Process the packet.
		utils.PrintDebug("gossip", l.GetAddress(), "is about to process a rumor from", rumor.Origin, "with sequence", rumor.Sequence, "of type", newPkt.Msg.Type)
		err := l.config.MessageRegistry.ProcessPacket(newPkt)
		utils.PrintDebug("gossip", l.GetAddress(), "has processed the", newPkt.Msg.Type)
		if err != nil {
			return fmt.Errorf("could not process the rumor packet: %w", err)
		}
	}
	// Relay the rumors message to a different random neighbor if it contains at least one new rumor.
	if len(rumorsOfInterest) > 0 {
		err := l.sendRumors(*rumorsMsg, map[string]struct{}{pkt.Header.Source: {}})
		if err != nil {
			return err
		}
	}
	// Send back AckMessage to the source after handling is done.
	// Create the ack message.
	ackMsg := types.AckMessage{}
	ackMsg.AckedPacketID = pkt.Header.PacketID
	ackMsg.Status = l.view.AsStatusMsg()
	if l.cryptography != nil {
		for _, ip := range l.cryptography.GetBlockedIPs() {
			ackMsg.Status[ip] = -1
		}
	}
	// Convert it into a transport message.
	ackTranspMsg, err := l.config.MessageRegistry.MarshalMessage(&ackMsg)
	if err != nil {
		return fmt.Errorf("could not marshal ack into a transport msg: %w", err)
	}
	// Send back the Acknowledgement.
	utils.PrintDebug("gossip", l.GetAddress(), "is about to acknowledge packet", ackMsg.AckedPacketID, "to", pkt.Header.RelayedBy)

	if l.cryptography != nil {
		return l.cryptography.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, ackTranspMsg)
	}
	return l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, ackTranspMsg)
}

func (l *Layer) StatusMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("gossip", l.GetAddress(), "is at StatusMessageHandler")
	statusMsg, ok := msg.(*types.StatusMessage)
	if !ok {
		return fmt.Errorf("could not parse status message")
	}
	// rmtNews contains the rumors that are new to me.
	// thsNews contains the rumors that are new to the remote node.
	rmtNews, thsNews := l.view.Compare(SeqMap(*statusMsg))
	utils.PrintDebug("gossip", l.GetAddress(), "found the following differences:", "\nrmtNews:", rmtNews, "\nthsNews:", thsNews)
	// Send back the missing rumors.
	if len(thsNews) > 0 {
		rumorsMsg := types.RumorsMessage{}
		// Get the remote's missing rumors from my rumor list.
		for origin, newSequence := range thsNews {
			// Note that the origin of the status message already has all the rumors that originate from itself.
			// Thus, we do not need to send back rumors originating from the sender.
			if origin == pkt.Header.Source {
				continue
			}
			sequenceMin, ok := (*statusMsg)[origin]
			// Iterate from the old sequence + 1 up to the new sequence (inclusive).
			i := sequenceMin + 1
			if !ok {
				i = 1
			}
			for i <= newSequence {
				savedRumor, ok := l.view.GetSavedRumor(origin, i)
				if !ok {
					return fmt.Errorf("the new rumor from this peer could not be found in the saved rumors list")
				}
				rumorsMsg.Rumors = append(rumorsMsg.Rumors, savedRumor)
				i += 1
			}
		}
		utils.PrintDebug("gossip", l.GetAddress(), "has collected", len(rumorsMsg.Rumors), "many rumors")
		trnspMsg, err := l.config.MessageRegistry.MarshalMessage(&rumorsMsg)
		if err != nil {
			return fmt.Errorf("could not convert the rumor message to a transport message: %w", err)
		}
		// Send the missing rumors back and do not wait for ack.
		if l.cryptography != nil {
			return l.cryptography.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, trnspMsg)
		} else {
			return l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, trnspMsg)
		}
	}
	// Remove blocked user's IPs from rmtNews
	if l.cryptography != nil && l.cryptography.HasBlockedIPs() {
		for k := range rmtNews {
			if l.cryptography.IsBlockedIP(k) {
				delete(rmtNews, k) // remove blocked users entries to avoid requesting for it
			}
		}
	}
	// Request my missing rumors from the remote peer after I make sure that he is up to date.
	if len(rmtNews) > 0 {
		myStatusMsg := l.view.AsStatusMsg()
		if l.cryptography != nil {
			for _, ip := range l.cryptography.GetBlockedIPs() {
				myStatusMsg[ip] = -1
			}
		}
		trnspMsg, _ := l.config.MessageRegistry.MarshalMessage(&myStatusMsg)
		if l.cryptography != nil {
			_ = l.cryptography.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, trnspMsg)
		} else {
			_ = l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, pkt.Header.RelayedBy, trnspMsg)
		}
	}
	// ContinueMongering process.
	if len(thsNews) == 0 && len(rmtNews) == 0 {
		utils.PrintDebug("gossip", l.GetAddress(), "is throwing a coin to continue mongering, p =",
			l.config.ContinueMongering)
		if rand.Float64() < l.config.ContinueMongering {
			utils.PrintDebug("gossip", l.GetAddress(), "is continuing mongering.")
			dest, err := l.network.ChooseRandomNeighbor(map[string]struct{}{pkt.Header.RelayedBy: {}})
			if err != nil {
				utils.PrintDebug("gossip", l.GetAddress(), "has stopped mongering since there are no neighbors to choose from.")
				return nil
			}
			myStatusMsg := l.view.AsStatusMsg()
			if l.cryptography != nil {
				for _, ip := range l.cryptography.GetBlockedIPs() {
					myStatusMsg[ip] = -1
				}
			}
			trnspMsg, _ := l.config.MessageRegistry.MarshalMessage(&myStatusMsg)
			if l.cryptography != nil {
				_ = l.cryptography.Route(l.GetAddress(), dest, dest, trnspMsg)
			} else {
				_ = l.network.Route(l.GetAddress(), dest, dest, trnspMsg)
			}

		}
		utils.PrintDebug("gossip", l.GetAddress(), "is stopping mongering.")
	}
	return nil
}

func (l *Layer) AckMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("gossip", l.GetAddress(), "is at AckMessageHandler")
	ackMsg, ok := msg.(*types.AckMessage)
	if !ok {
		return fmt.Errorf("could not parse the ack message")
	}
	_ = l.ackNotification.DispatchResponse(ackMsg.AckedPacketID, *ackMsg)
	// Now, process the status message.
	transpMsg, err := l.config.MessageRegistry.MarshalMessage(&ackMsg.Status)
	if err != nil {
		return fmt.Errorf("could not marshal the embedded status message in the ack message: %w", err)
	}
	transpPacket := transport.Packet{
		Header: pkt.Header,
		Msg:    &transpMsg,
	}
	_ = l.config.MessageRegistry.ProcessPacket(transpPacket.Copy())
	return nil
}

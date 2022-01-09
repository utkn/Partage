package paxos

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type BlockGenerator = func(types.PaxosAcceptMessage) types.BlockchainBlock
type BlockchainUpdater = func(types.BlockchainBlock)
type ProposalChecker = func(types.PaxosProposeMessage) bool

type Acceptor struct {
	paxos *Paxos
	// Generates a block from an accept message.
	BlockGenerator
	// Updates the blockchain with a generated block.
	BlockchainUpdater
	// Checks whether a given proposal is valid. The proposal is discarded if this returns false.
	ProposalChecker
}

func (a *Acceptor) HandlePrepare(msg types.PaxosPrepareMessage) error {
	utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is handling paxos prepare for ID", msg.ID)
	a.paxos.Clock.Lock.Lock()
	// Ignore when receivedStep != clock.Step || receivedID <= clock.MaxID
	if a.paxos.Clock.ShouldIgnorePrepare(msg.Step, int(msg.ID)) {
		utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), a.paxos.Clock.String(), "ignored the prepare.")
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	// Make sure that we are up-to-date.
	a.paxos.Clock.UpdateMaxID(int(msg.ID))
	promiseMsg := types.PaxosPromiseMessage{
		Step:          msg.Step,
		ID:            msg.ID,
		AcceptedID:    0,
		AcceptedValue: nil,
	}
	// If we have previously accepted a value in this step, inform the proposer.
	if a.paxos.Clock.AcceptedValue != nil {
		promiseMsg.AcceptedID = a.paxos.Clock.AcceptedID
		promiseMsg.AcceptedValue = a.paxos.Clock.AcceptedValue
		utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is informing the proposer of an already accepted value",
			a.paxos.Clock.AcceptedValue, "at step", a.paxos.Clock.AcceptedID)
	}
	a.paxos.Clock.Lock.Unlock()
	promiseTransportMsg, _ := a.paxos.Config.MessageRegistry.MarshalMessage(&promiseMsg)
	// Wrap the transport msg in a consensus msg.
	promiseTransportMsg = protocol.WrapInConsensusPacket(a.paxos.ProtocolID, a.paxos.Config, promiseTransportMsg)
	// Wrap the consensus msg in a private msg.
	privateMsg := types.PrivateMessage{
		Recipients: map[string]struct{}{msg.Source: {}},
		Msg:        &promiseTransportMsg,
	}
	// Send back the promise.
	privateTransportMsg, _ := a.paxos.Config.MessageRegistry.MarshalMessage(&privateMsg)
	utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is sending back a promise for ID", msg.ID)
	return a.paxos.Gossip.Broadcast(privateTransportMsg)
}

func (a *Acceptor) HandlePropose(msg types.PaxosProposeMessage) error {
	utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is handling paxos propose for ID", msg.ID)
	a.paxos.Clock.Lock.Lock()
	// Ignore when receivedStep != clock.Step || receivedID != clock.MaxID
	if a.paxos.Clock.ShouldIgnorePropose(msg.Step, int(msg.ID)) {
		utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), a.paxos.Clock, "ignored the proposal, since it is in step")
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	// OR ignore when the proposal checker returns false.
	if !a.ProposalChecker(msg) {
		utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), a.paxos.Clock, "ignored the proposal, since the checker returned false")
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	// Accept the value and save in the clock.
	utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is accepting by setting its accepted ID to", msg.ID)
	a.paxos.Clock.Accept(msg.ID, msg.Value)
	a.paxos.Clock.Lock.Unlock()
	acceptMsg := types.PaxosAcceptMessage(msg)
	utils.PrintDebug("acceptor", a.paxos.Gossip.GetAddress(), "is sending back an accept for ID", msg.ID)
	acceptTranspMsg, _ := a.paxos.Config.MessageRegistry.MarshalMessage(&acceptMsg)
	// Broadcast accept messages.
	return a.paxos.Gossip.Broadcast(protocol.WrapInConsensusPacket(a.paxos.ProtocolID, a.paxos.Config, acceptTranspMsg))
}

func (a *Acceptor) HandleTLC(msg types.TLCMessage) error {
	utils.PrintDebug("tlc", a.paxos.Gossip.GetAddress(), "is handling a TLC message")
	a.paxos.Clock.Lock.Lock()
	// Do not consider old blocks.
	if msg.Step < a.paxos.Clock.Step {
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	// Save the received TLC message.
	a.paxos.Clock.NotifyTLC(int(msg.Step), msg.Block)
	utils.PrintDebug("tlc",
		a.paxos.Gossip.GetAddress(),
		"has incremented its counter to",
		a.paxos.Clock.TLCProgressMap[int(msg.Step)].Progress,
		"at step",
		int(msg.Step),
	)
	// Get the list of new blocks that should be appended. Move the clock in the meantime.
	newBlocks := a.paxos.Clock.CatchUp(a.paxos.Config.PaxosThreshold(a.paxos.Config.TotalPeers))
	newStep := a.paxos.Clock.Step
	utils.PrintDebug("tlc", a.paxos.Gossip.GetAddress(), "will be appending", len(newBlocks), "new blocks.")
	for _, newBlock := range newBlocks {
		a.BlockchainUpdater(newBlock)
	}
	// If we have not added new blocks, then we did not move the clock at all.
	if len(newBlocks) == 0 {
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	//println("tlc", a.gossip.GetAddress(), "has appended", len(newBlocks), "blocks", newBlocks[0].Value.String())
	// At this point, we are sure that we have moved the clock.
	// Try to broadcast *only* for this step if we haven't done so yet.
	if !a.paxos.Clock.HasBroadcasted(int(msg.Step)) {
		a.paxos.Clock.MarkBroadcasted(int(msg.Step))
		a.paxos.Clock.Lock.Unlock()
		tlcMsgCopy := types.TLCMessage{
			Step:  msg.Step,
			Block: msg.Block,
		}
		tlcTranspMsg, _ := a.paxos.Config.MessageRegistry.MarshalMessage(&tlcMsgCopy)
		utils.PrintDebug("tlc", a.paxos.Gossip.GetAddress(), "is broadcasting away TLC messages for step", msg.Step)
		//println(a.gossip.GetAddress(), "is broadcasting TLC for value", tlcMsgCopy.Block.Value.String(), "for step", msg.Step)
		_ = a.paxos.Gossip.Broadcast(protocol.WrapInConsensusPacket(a.paxos.ProtocolID, a.paxos.Config, tlcTranspMsg))
	} else {
		a.paxos.Clock.Lock.Unlock()
		utils.PrintDebug("tlc", a.paxos.Gossip.GetAddress(), "is bypassing broadcast for step", msg.Step)
	}
	// Inform the local proposer that we have moved the clock.
	a.paxos.Notification.DispatchResponse(fmt.Sprint("tick", msg.Step), types.EmptyMessage{})
	utils.PrintDebug("tlc", a.paxos.Gossip.GetAddress(), "has updated its clock step to", newStep)
	return nil
}

func (a *Acceptor) HandleAccept(msg types.PaxosAcceptMessage) error {
	utils.PrintDebug("proposer", a.paxos.Gossip.GetAddress(), "is handling paxos accept for ID", msg.ID)
	a.paxos.Clock.Lock.Lock()
	// Do not consider accept messages for an invalid step or ID.
	if a.paxos.Clock.ShouldIgnorePropose(msg.Step, int(msg.ID)) {
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	// Let the proposer handle the acceptance message in the background.
	a.paxos.Notification.DispatchResponse(fmt.Sprint("accept-id", msg.ID), msg)
	// Save the received acceptance message.
	reachedThreshold := a.paxos.Clock.NotifyAcceptance(msg.Value, a.paxos.Config.PaxosThreshold(a.paxos.Config.TotalPeers))
	// If we haven't reached the threshold, or we have already broadcasted a TLC message, leave it be.
	if !reachedThreshold || a.paxos.Clock.HasBroadcasted(int(msg.Step)) {
		a.paxos.Clock.Lock.Unlock()
		return nil
	}
	a.paxos.Clock.MarkBroadcasted(int(msg.Step))
	a.paxos.Clock.Lock.Unlock()
	// If we finally reached a threshold, broadcast a TLC message.
	// To do that, first construct the blockchain block.
	block := a.BlockGenerator(msg)
	// Create the TLC message.
	tlcMessage := types.TLCMessage{
		Step:  msg.Step,
		Block: block,
	}
	tlcTranspMessage, _ := a.paxos.Config.MessageRegistry.MarshalMessage(&tlcMessage)
	utils.PrintDebug("proposer", a.paxos.Gossip.GetAddress(), "is broadcasting TLC for value", tlcMessage.Block.Value.String(),
		"for step", msg.Step, "from accepthandler")
	return a.paxos.Gossip.Broadcast(protocol.WrapInConsensusPacket(a.paxos.ProtocolID, a.paxos.Config, tlcTranspMessage))
}

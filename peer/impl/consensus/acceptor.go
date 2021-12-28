package consensus

import (
	"encoding/hex"
	"fmt"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

type Acceptor struct {
	clock        *Clock
	config       *peer.Configuration
	gossip       *gossip.Layer
	notification *utils.AsyncNotificationHandler
}

func (a *Acceptor) HandlePrepare(msg types.PaxosPrepareMessage) error {
	utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is handling paxos prepare for ID", msg.ID)
	a.clock.Lock.Lock()
	// Ignore when receivedStep != clock.Step || receivedID <= clock.MaxID
	if a.clock.ShouldIgnorePrepare(msg.Step, int(msg.ID)) {
		utils.PrintDebug("acceptor", a.gossip.GetAddress(), a.clock.String(), "ignored the prepare.")
		a.clock.Lock.Unlock()
		return nil
	}
	// Make sure that we are up-to-date.
	a.clock.UpdateMaxID(int(msg.ID))
	promiseMsg := types.PaxosPromiseMessage{
		Step:          msg.Step,
		ID:            msg.ID,
		AcceptedID:    0,
		AcceptedValue: nil,
	}
	// If we have previously accepted a value in this step, inform the proposer.
	if a.clock.AcceptedValue != nil {
		promiseMsg.AcceptedID = a.clock.AcceptedID
		promiseMsg.AcceptedValue = a.clock.AcceptedValue
		utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is informing the proposer of an already accepted value",
			a.clock.AcceptedValue, "at step", a.clock.AcceptedID)
	}
	a.clock.Lock.Unlock()
	promiseTransportMsg, _ := a.config.MessageRegistry.MarshalMessage(&promiseMsg)
	privateMsg := types.PrivateMessage{
		Recipients: map[string]struct{}{msg.Source: {}},
		Msg:        &promiseTransportMsg,
	}
	// Send back the promise.
	privateTransportMsg, _ := a.config.MessageRegistry.MarshalMessage(&privateMsg)
	utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is sending back a promise for ID", msg.ID)
	return a.gossip.Broadcast(privateTransportMsg)
}

func (a *Acceptor) HandlePropose(msg types.PaxosProposeMessage) error {
	utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is handling paxos propose for ID", msg.ID)
	a.clock.Lock.Lock()
	// Ignore when receivedStep != clock.Step || receivedID != clock.MaxID
	if a.clock.ShouldIgnorePropose(msg.Step, int(msg.ID)) {
		utils.PrintDebug("acceptor", a.gossip.GetAddress(), a.clock, "ignored the proposal, since it is in step")
		a.clock.Lock.Unlock()
		return nil
	}
	// Accept the value and save in the clock.
	utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is accepting by setting its accepted ID to", msg.ID)
	a.clock.Accept(msg.ID, msg.Value)
	a.clock.Lock.Unlock()
	acceptMsg := types.PaxosAcceptMessage(msg)
	utils.PrintDebug("acceptor", a.gossip.GetAddress(), "is sending back an accept for ID", msg.ID)
	acceptTranspMsg, _ := a.config.MessageRegistry.MarshalMessage(&acceptMsg)
	// Broadcast accept messages.
	return a.gossip.Broadcast(acceptTranspMsg)
}

func (a *Acceptor) HandleTLC(msg types.TLCMessage) error {
	utils.PrintDebug("tlc", a.gossip.GetAddress(), "is handling a TLC message")
	a.clock.Lock.Lock()
	// Do not consider old blocks.
	if msg.Step < a.clock.Step {
		a.clock.Lock.Unlock()
		return nil
	}
	// Save the received TLC message.
	a.clock.NotifyTLC(int(msg.Step), msg.Block)
	utils.PrintDebug("tlc",
		a.gossip.GetAddress(),
		"has incremented its counter to",
		a.clock.TLCProgressMap[int(msg.Step)].Progress,
		"at step",
		int(msg.Step),
	)
	// Get the list of new blocks that should be appended. Move the clock in the meantime.
	newBlocks := a.clock.CatchUp(a.config.PaxosThreshold(a.config.TotalPeers))
	newStep := a.clock.Step
	utils.PrintDebug("tlc", a.gossip.GetAddress(), "will be appending", len(newBlocks), "new blocks.")
	for _, newBlock := range newBlocks {
		newBlockBytes, _ := newBlock.Marshal()
		a.config.Storage.GetBlockchainStore().Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		a.config.Storage.GetBlockchainStore().Set(newBlockHash, newBlockBytes)
		a.config.Storage.GetNamingStore().Set(newBlock.Value.Filename, []byte(newBlock.Value.Metahash))
	}
	// If we have not added new blocks, then we did not move the clock at all.
	if len(newBlocks) == 0 {
		a.clock.Lock.Unlock()
		return nil
	}
	//println("tlc", a.gossip.GetAddress(), "has appended", len(newBlocks), "blocks", newBlocks[0].Value.String())
	// At this point, we are sure that we have moved the clock.
	// Try to broadcast *only* for this step if we haven't done so yet.
	if !a.clock.HasBroadcasted(int(msg.Step)) {
		a.clock.MarkBroadcasted(int(msg.Step))
		a.clock.Lock.Unlock()
		tlcMsgCopy := types.TLCMessage{
			Step:  msg.Step,
			Block: msg.Block,
		}
		tlcTranspMsg, _ := a.config.MessageRegistry.MarshalMessage(&tlcMsgCopy)
		utils.PrintDebug("tlc", a.gossip.GetAddress(), "is broadcasting away TLC messages for step", msg.Step)
		//println(a.gossip.GetAddress(), "is broadcasting TLC for value", tlcMsgCopy.Block.Value.String(), "for step", msg.Step)
		_ = a.gossip.Broadcast(tlcTranspMsg)
	} else {
		a.clock.Lock.Unlock()
		utils.PrintDebug("tlc", a.gossip.GetAddress(), "is bypassing broadcast for step", msg.Step)
	}
	// Inform the local proposer that we have moved the clock.
	a.notification.DispatchResponse(fmt.Sprint("tick", msg.Step), types.EmptyMessage{})
	utils.PrintDebug("tlc", a.gossip.GetAddress(), "has updated its clock step to", newStep)
	return nil
}

func (p *Acceptor) HandleAccept(msg types.PaxosAcceptMessage) error {
	utils.PrintDebug("proposer", p.gossip.GetAddress(), "is handling paxos accept for ID", msg.ID)
	p.clock.Lock.Lock()
	// Do not consider accept messages for an invalid step or ID.
	if p.clock.ShouldIgnorePropose(msg.Step, int(msg.ID)) {
		p.clock.Lock.Unlock()
		return nil
	}
	// Let the proposer handle the acceptance message in the background.
	p.notification.DispatchResponse(fmt.Sprint("accept-id", msg.ID), msg)
	// Save the received acceptance message.
	reachedThreshold := p.clock.NotifyAcceptance(msg.Value, p.config.PaxosThreshold(p.config.TotalPeers))
	// If we haven't reached the threshold, or we have already broadcasted a TLC message, leave it be.
	if !reachedThreshold || p.clock.HasBroadcasted(int(msg.Step)) {
		p.clock.Lock.Unlock()
		return nil
	}
	p.clock.MarkBroadcasted(int(msg.Step))
	p.clock.Lock.Unlock()
	// If we finally reached a threshold, broadcast a TLC message.
	// To do that, first construct the blockchain block.
	prevHash := make([]byte, 32)
	lastBlockHashBytes := p.config.Storage.GetBlockchainStore().Get(storage.LastBlockKey)
	lastBlockHash := hex.EncodeToString(lastBlockHashBytes)
	lastBlockBuf := p.config.Storage.GetBlockchainStore().Get(lastBlockHash)
	if lastBlockBuf != nil {
		var lastBlock types.BlockchainBlock
		_ = lastBlock.Unmarshal(lastBlockBuf)
		prevHash = lastBlock.Hash
	}
	// Create the block hash.
	blockHash := utils.HashBlock(
		int(msg.Step),
		msg.Value.UniqID,
		msg.Value.Filename,
		msg.Value.Metahash,
		prevHash,
	)
	// Create the block.
	block := types.BlockchainBlock{
		Index:    msg.Step,
		Hash:     blockHash,
		Value:    msg.Value,
		PrevHash: prevHash,
	}
	// Create the TLC message.
	tlcMessage := types.TLCMessage{
		Step:  msg.Step,
		Block: block,
	}
	tlcTranspMessage, _ := p.config.MessageRegistry.MarshalMessage(&tlcMessage)
	utils.PrintDebug("proposer", p.gossip.GetAddress(), "is broadcasting TLC for value", tlcMessage.Block.Value.String(),
		"for step", msg.Step, "from accepthandler")
	return p.gossip.Broadcast(tlcTranspMessage)
}

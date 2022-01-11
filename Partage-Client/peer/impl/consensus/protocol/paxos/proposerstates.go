package paxos

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type ProposerBeginState struct {
	State
	paxos *Paxos
	value types.PaxosValue
}

type ProposerWaitPromiseState struct {
	State
	paxos         *Paxos
	proposalID    uint
	proposalStep  uint
	originalValue types.PaxosValue
	notification  *utils.AsyncNotificationHandler
}

type ProposerWaitAcceptState struct {
	State
	paxos         *Paxos
	notification  *utils.AsyncNotificationHandler
	proposalStep  uint
	proposalID    uint
	chosenValue   types.PaxosValue
	originalValue types.PaxosValue
}

type ProposerDoneState struct {
	State
	paxos         *Paxos
	proposalStep  uint
	proposalID    uint
	proposedValue types.PaxosValue
	originalValue types.PaxosValue
}

func (s ProposerBeginState) Next() (State, types.BlockchainBlock) {
	s.paxos.Clock.Lock.RLock()
	// Catch up with the clock!
	for int(s.paxos.LastProposalID) < s.paxos.Clock.MaxID {
		s.paxos.LastProposalID += s.paxos.Config.TotalPeers
	}
	proposalID := s.paxos.LastProposalID
	proposalStep := s.paxos.Clock.Step
	s.paxos.Clock.Lock.RUnlock()
	//println("proposer", p.gossip.GetAddress(), "is proposing", value.String(), "with ID", proposalID, "at step", proposalStep)
	// Update the next proposal ID.
	s.paxos.LastProposalID += s.paxos.Config.TotalPeers
	return ProposerWaitPromiseState{
		paxos:         s.paxos,
		proposalID:    proposalID,
		proposalStep:  proposalStep,
		notification:  utils.NewAsyncNotificationHandler(),
		originalValue: s.value,
	}, types.BlockchainBlock{}
}

func (s ProposerBeginState) Accept(message types.Message) bool {
	return false
}

func (s ProposerBeginState) Name() string {
	return "ProposerBegin"
}

func (s ProposerWaitPromiseState) Next() (State, types.BlockchainBlock) {
	// First, broadcast the prepare-message.
	prepareMsg := types.PaxosPrepareMessage{
		Step:   s.proposalStep,
		ID:     s.proposalID,
		Source: s.paxos.Gossip.GetAddress(),
	}
	// Pass the created prepare message to the next state.
	prepareTranspMsg, _ := s.paxos.Config.MessageRegistry.MarshalMessage(&prepareMsg)
	// Broadcast the prepare message.
	_ = s.paxos.Gossip.BroadcastMessage(protocol.WrapInConsensusMessage(s.paxos.ProtocolID, prepareTranspMsg))
	// Find the threshold.
	threshold := s.paxos.Config.PaxosThreshold(s.paxos.Config.TotalPeers)
	// Collect the promises in the background.
	var promises []*types.PaxosPromiseMessage
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has started waiting for paxos promises with ID", s.proposalID)
	responses := s.notification.MultiResponseCollector(fmt.Sprint("proposer-promise-id", s.proposalID), s.paxos.Config.PaxosProposerRetry, threshold)
	for _, r := range responses {
		promises = append(promises, r.(*types.PaxosPromiseMessage))
	}
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has received", len(promises), "promises with ID", s.proposalID)
	// Retry with new proposal ID.
	if len(promises) < threshold {
		utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "couldn't collect enough promises.")
		//println(p.gossip.GetAddress(), "NOT ENOUGH PROMISES")
		return ProposerBeginState{
			paxos: s.paxos,
			value: s.originalValue,
		}, types.BlockchainBlock{}
	}
	// Get the highest last accepted value if it exists.
	var alreadyAcceptedValue *types.PaxosValue
	alreadyAcceptedID := -1
	for _, promiseMsg := range promises {
		if promiseMsg.AcceptedValue == nil {
			continue
		}
		if int(promiseMsg.AcceptedID) > alreadyAcceptedID {
			alreadyAcceptedValue = promiseMsg.AcceptedValue
		}
	}
	// Choose either the original value or the already accepted value.
	chosenValue := s.originalValue
	if alreadyAcceptedValue != nil {
		utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "is switching to propose for", alreadyAcceptedValue)
		chosenValue = *alreadyAcceptedValue
	}
	return ProposerWaitAcceptState{
		paxos:         s.paxos,
		originalValue: s.originalValue,
		chosenValue:   chosenValue,
		proposalID:    s.proposalID,
		proposalStep:  s.proposalStep,
		notification:  utils.NewAsyncNotificationHandler(),
	}, types.BlockchainBlock{}
}

func (s ProposerWaitPromiseState) Accept(message types.Message) bool {
	promiseMsg, ok := message.(*types.PaxosPromiseMessage)
	if !ok {
		//println("Proposer dropping irrelevant msg.")
		return false
	}
	if promiseMsg.ID != s.proposalID {
		//println("Proposer dropping promise msg.")
		return false
	}
	s.notification.DispatchResponse(fmt.Sprint("proposer-promise-id", promiseMsg.ID), promiseMsg)
	return true
}

func (s ProposerWaitPromiseState) Name() string {
	return "ProposerWaitPromise"
}

func (s ProposerWaitAcceptState) Next() (State, types.BlockchainBlock) {
	threshold := s.paxos.Config.PaxosThreshold(s.paxos.Config.TotalPeers)
	proposeMsg := types.PaxosProposeMessage{
		Step:   s.proposalStep,
		ID:     s.proposalID,
		Value:  s.chosenValue,
		Source: s.paxos.Gossip.GetAddress(),
	}
	proposeTranspMsg, _ := s.paxos.Config.MessageRegistry.MarshalMessage(&proposeMsg)
	// Broadcast the proposal.
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "is broadcasting a propose for ID", s.proposalID, "and value", s.chosenValue)
	_ = s.paxos.Gossip.BroadcastMessage(protocol.WrapInConsensusMessage(s.paxos.ProtocolID, proposeTranspMsg))
	// Collect accept messages.
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has started waiting for paxos accepts with ID", s.proposalID)
	var accepts []*types.PaxosAcceptMessage
	responses := s.notification.MultiResponseCollector(fmt.Sprint("proposer-accept-id", proposeMsg.ID), s.paxos.Config.PaxosProposerRetry, threshold)
	receivedRejects := 0
	for _, r := range responses {
		acceptMsg := r.(*types.PaxosAcceptMessage)
		// Do not consider irrelevant accept messages.
		if acceptMsg.Value.UniqID != s.chosenValue.UniqID {
			continue
		}
		// Do not consider reject messages. Keep track of the # received.
		if content.IsReject(acceptMsg.Value.CustomValue) {
			receivedRejects += 1
			continue
		}
		accepts = append(accepts, acceptMsg)
	}
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has received", len(accepts), "paxos accepts with ID", s.proposalID)
	// Retry with new proposal ID if we couldn't collect enough accepts.
	if len(accepts) < threshold {
		// If the majority of the network has rejected our request, the issue is probably on our end...
		// Prematurely terminate the proposal.
		if receivedRejects >= threshold {
			utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has received", receivedRejects, "rejects and terminating!")
			return nil, types.BlockchainBlock{}
		}
		utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "couldn't collect enough accepts. Retrying...")
		//println(p.gossip.GetAddress(), "NOT ENOUGH ACCEPTS")
		return ProposerBeginState{
			paxos: s.paxos,
			value: s.originalValue,
		}, types.BlockchainBlock{}
	}
	return ProposerDoneState{
		paxos:         s.paxos,
		proposalStep:  s.proposalStep,
		proposalID:    s.proposalID,
		proposedValue: s.chosenValue,
		originalValue: s.originalValue,
	}, types.BlockchainBlock{}
}

func (s ProposerWaitAcceptState) Accept(message types.Message) bool {
	acceptMsg, ok := message.(*types.PaxosAcceptMessage)
	if !ok {
		return false
	}
	if acceptMsg.ID != s.proposalID {
		return false
	}
	s.notification.DispatchResponse(fmt.Sprint("proposer-accept-id", acceptMsg.ID), acceptMsg)
	return true
}

func (s ProposerWaitAcceptState) Name() string {
	return "ProposerWaitAccept"
}

func (s ProposerDoneState) Next() (State, types.BlockchainBlock) {
	// Listen to the tick from the global notification handler.
	tlcMsg := s.paxos.Notification.ResponseCollector(fmt.Sprint("tick", s.proposalStep), s.paxos.Config.PaxosProposerRetry)
	// Retry if the whole proposal has timed out.
	if tlcMsg == nil {
		utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "timed out for some reason. Retrying...")
		//println(p.gossip.GetAddress(), "NO TICK!")
		return ProposerBeginState{
			paxos: s.paxos,
		}, types.BlockchainBlock{}
	}
	// Retry if the proposed value is not ours.
	if s.originalValue.UniqID != s.proposedValue.UniqID {
		utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "will now retry to propose its own value.")
		return ProposerBeginState{
			paxos: s.paxos,
		}, types.BlockchainBlock{}
	}
	utils.PrintDebug("proposer", s.paxos.Gossip.GetAddress(), "has concluded the proposal with ID",
		s.proposalID, "and value", s.proposedValue.String())
	// Stop the state machine and return the agreed block.
	return nil, tlcMsg.(types.TLCMessage).Block
}

func (s ProposerDoneState) Accept(message types.Message) bool {
	return false
}

func (s ProposerDoneState) Name() string {
	return "ProposerDone"
}

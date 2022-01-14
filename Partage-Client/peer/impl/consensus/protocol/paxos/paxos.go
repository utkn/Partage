package paxos

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Paxos struct {
	protocol.Protocol
	proposalLock   sync.Mutex
	handleLock     sync.RWMutex
	ProtocolID     string
	Clock          *Clock
	acceptor       *Acceptor
	Proposer       *StateMachine
	LastProposalID uint

	Notification *utils.AsyncNotificationHandler
	Gossip       *gossip.Layer
	Config       *peer.Configuration
}

func New(protocolID string, config *peer.Configuration, gossip *gossip.Layer,
	blockGenerator BlockGenerator,
	blockchainUpdater BlockchainUpdater,
	proposalChecker ProposalChecker) *Paxos {
	p := Paxos{
		ProtocolID:     protocolID,
		Clock:          NewClock(),
		Proposer:       &StateMachine{},
		LastProposalID: config.PaxosID,

		Notification: utils.NewAsyncNotificationHandler(),
		Gossip:       gossip,
		Config:       config,
	}
	// Create the acceptor. Acceptor methods will be invoked by the message handlers.
	p.acceptor = &Acceptor{
		paxos:             &p,
		BlockGenerator:    blockGenerator,
		BlockchainUpdater: blockchainUpdater,
		ProposalChecker:   proposalChecker,
	}
	return &p
}

func (p *Paxos) GetProtocolID() string {
	return p.ProtocolID
}

func (p *Paxos) Propose(val types.PaxosValue) (string, error) {
	p.proposalLock.Lock()
	defer p.proposalLock.Unlock()
	outputBlock := p.Proposer.Run(ProposerBeginState{
		paxos: p,
		value: val,
	})
	// If the proposer returns an empty block, this means that the proposal has failed.
	if outputBlock.Value.UniqID == "" {
		return "", fmt.Errorf("proposal was rejected at the consensus layer")
	}
	return hex.EncodeToString(outputBlock.Hash), nil
}

func (p *Paxos) HandleConsensusMessage(msg protocol.ConsensusMessage) error {
	p.handleLock.RLock()
	defer p.handleLock.RUnlock()
	innerMsg := protocol.UnwrapConsensusMessage(msg)
	switch innerMsg.Name() {
	case "paxosaccept":
		p.Proposer.Input(innerMsg)
		acceptMsg := innerMsg.(*types.PaxosAcceptMessage)
		return p.acceptor.HandleAccept(*acceptMsg)
	case "paxospromise":
		p.Proposer.Input(innerMsg)
		return nil
	case "paxosprepare":
		prepareMsg := innerMsg.(*types.PaxosPrepareMessage)
		return p.acceptor.HandlePrepare(*prepareMsg)
	case "paxospropose":
		proposeMsg := innerMsg.(*types.PaxosProposeMessage)
		return p.acceptor.HandlePropose(*proposeMsg)
	case "tlc":
		tlcMsg := innerMsg.(*types.TLCMessage)
		return p.acceptor.HandleTLC(*tlcMsg)
	}
	return nil
}

func (p *Paxos) UpdateSystemSize(oldSize uint, newSize uint) error {
	if newSize < oldSize {
		return fmt.Errorf("cannot decrease system size in Paxos")
	}
	if p.ProtocolID != "registration" {
		// Wait for the proposal to finish before updating the system size.
		p.proposalLock.Lock()
		defer p.proposalLock.Unlock()
		// Wait for the message handling to finish before updating the system size.
		p.handleLock.Lock()
		defer p.handleLock.Unlock()
	}
	// Update the last proposal ID.
	if p.LastProposalID > oldSize {
		incrementAmount := (p.LastProposalID - 1) / oldSize
		diff := newSize - oldSize
		p.LastProposalID += diff * incrementAmount
	}
	return nil
}

func (p *Paxos) LocalUpdate(value types.PaxosValue) (string, error) {
	return "", nil
}

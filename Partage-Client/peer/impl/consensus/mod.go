package consensus

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"sync"
)

type Layer struct {
	sync.RWMutex
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
	layer.RegisterProtocol("default",
		paxos.New("default", config, gossip,
			DefaultBlockGenerator(config.Storage.GetBlockchainStore()),
			DefaultBlockchainUpdater(config.Storage.GetBlockchainStore(), config.Storage.GetNamingStore()),
			DefaultProposalChecker()))
	return layer
}

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(protocol.ConsensusMessage{}, l.HandleConsensusMessage)
}

func (l *Layer) GetAddress() string {
	return l.Gossip.GetAddress()
}

func (l *Layer) RegisterProtocol(id string, protocol protocol.Protocol) {
	l.Lock()
	defer l.Unlock()
	l.protocols[id] = protocol
}

func (l *Layer) IsRegistered(id string) bool {
	l.RLock()
	defer l.RUnlock()
	_, ok := l.protocols[id]
	return ok
}

// Propose proposes the given value with the default protocol.
func (l *Layer) Propose(value types.PaxosValue) error {
	return l.ProposeWithProtocol("default", value)
}

// ProposeWithProtocol proposes the given value with the protocol associated with the given protocol id.
func (l *Layer) ProposeWithProtocol(protocolID string, value types.PaxosValue) error {
	// Consensus should not be invoked when there are <= 1 many peers.
	if l.Config.TotalPeers <= 1 {
		return fmt.Errorf("consensus is disabled for <= 1 many peers")
	}
	// Get the protocol
	l.RLock()
	p, ok := l.protocols[protocolID]
	if !ok {
		return fmt.Errorf("could not find the consensus protocol with id %s", protocolID)
	}
	l.RUnlock()
	// Initiate the Paxos consensus protocol.
	err := p.Propose(value)
	if err != nil {
		return err
	}
	return nil
}

// HandleConsensusMessage forwards a consensus message to registered protocols.
func (l *Layer) HandleConsensusMessage(msg types.Message, pkt transport.Packet) error {
	l.RLock()
	defer l.RUnlock()
	consensusMsg, ok := msg.(*protocol.ConsensusMessage)
	if !ok {
		return fmt.Errorf("could not parse the received consensus msg")
	}
	// Route the consensus message to the appropriate protocols. Let them decide what to do with it.
	for _, p := range l.protocols {
		// Skip the unrelated protocols.
		if p.GetProtocolID() != consensusMsg.ProtocolID {
			continue
		}
		err := p.HandleConsensusMessage(*consensusMsg)
		if err != nil {
			return err
		}
	}
	return nil
}

// -- Paxos functions for the default protocol.

func DefaultBlockGenerator(blockchainStore storage.Store) paxos.BlockGenerator {
	return func(msg types.PaxosAcceptMessage) types.BlockchainBlock {
		prevHash := make([]byte, 32)
		lastBlockHashBytes := blockchainStore.Get(storage.LastBlockKey)
		lastBlockHash := hex.EncodeToString(lastBlockHashBytes)
		lastBlockBuf := blockchainStore.Get(lastBlockHash)
		if lastBlockBuf != nil {
			var lastBlock types.BlockchainBlock
			_ = lastBlock.Unmarshal(lastBlockBuf)
			prevHash = lastBlock.Hash
		}
		// Create the block hash.
		blockHash := utils.HashNameBlock(
			int(msg.Step),
			msg.Value.UniqID,
			msg.Value.Filename,
			msg.Value.Metahash,
			prevHash,
		)
		// Create the block.
		return types.BlockchainBlock{
			Index:    msg.Step,
			Hash:     blockHash,
			Value:    msg.Value,
			PrevHash: prevHash,
		}
	}
}

func DefaultBlockchainUpdater(blockchainStore storage.Store, namingStore storage.Store) paxos.BlockchainUpdater {
	return func(newBlock types.BlockchainBlock) {
		newBlockBytes, _ := newBlock.Marshal()
		blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		blockchainStore.Set(newBlockHash, newBlockBytes)
		namingStore.Set(newBlock.Value.Filename, []byte(newBlock.Value.Metahash))
	}
}

func DefaultProposalChecker() paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		return true
	}
}
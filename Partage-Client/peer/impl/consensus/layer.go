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

func (l *Layer) GetAddress() string {
	return l.Gossip.GetAddress()
}

func (l *Layer) RegisterProtocol(id string, protocol protocol.Protocol) {
	utils.PrintDebug("consensus", l.GetAddress(), "is registering a new protocol", id)
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
func (l *Layer) Propose(value types.PaxosValue) (string, error) {
	return l.ProposeWithProtocol("default", value)
}

// ProposeWithProtocol proposes the given value with the protocol associated with the given protocol id.
// Returns the newly appended block hash.
func (l *Layer) ProposeWithProtocol(protocolID string, value types.PaxosValue) (string, error) {
	utils.PrintDebug("consensus", l.GetAddress(), "is proposing with", protocolID)
	// Get the protocol
	l.RLock()
	p, ok := l.protocols[protocolID]
	if !ok {
		return "", fmt.Errorf("could not find the consensus protocol with id %s", protocolID)
	}
	// Consensus should not be invoked when there are <= 1 many peers.
	//if l.Config.TotalPeers <= 1 {
	//	return "", fmt.Errorf("consensus is disabled for <= 1 many peers")
	//}
	l.RUnlock()
	// Initiate the Paxos consensus protocol.
	return p.Propose(value)
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
		blockchainStore.Set(storage.LastBlockKey, newBlock.Hash)
		newBlockHash := hex.EncodeToString(newBlock.Hash)
		newBlockBytes, _ := newBlock.Marshal()
		blockchainStore.Set(newBlockHash, newBlockBytes)
		namingStore.Set(newBlock.Value.Filename, []byte(newBlock.Value.Metahash))
	}
}

func DefaultProposalChecker() paxos.ProposalChecker {
	return func(msg types.PaxosProposeMessage) bool {
		return true
	}
}

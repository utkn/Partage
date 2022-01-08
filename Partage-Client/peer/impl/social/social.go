package social

import (
	"encoding/hex"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/data"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	consensus *consensus.Layer
	data      *data.Layer
}

func Construct(config *peer.Configuration, gossip *gossip.Layer, consensus *consensus.Layer) *Layer {
	consensus.RegisterProtocol("feed", paxos.New(config, gossip, FeedBlockFactory))
	return nil
}

func (l *Layer) GetAddress() string {
	return l.consensus.GetAddress()
}

func (l *Layer) PostContent(content string) {}

func (l *Layer) ReactToPost() {}

func (l *Layer) FollowUser() {}

func FeedBlockFactory(config *peer.Configuration, msg types.PaxosAcceptMessage) types.BlockchainBlock {
	prevHash := make([]byte, 32)
	lastBlockHashBytes := config.Storage.GetBlockchainStore().Get(storage.LastBlockKey)
	lastBlockHash := hex.EncodeToString(lastBlockHashBytes)
	lastBlockBuf := config.Storage.GetBlockchainStore().Get(lastBlockHash)
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
	return types.BlockchainBlock{
		Index:    msg.Step,
		Hash:     blockHash,
		Value:    msg.Value,
		PrevHash: prevHash,
	}
}

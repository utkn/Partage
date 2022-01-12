package social

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/data"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	consensus *consensus.Layer
	gossip    *gossip.Layer
	data      *data.Layer

	Config    *peer.Configuration
	FeedStore *feed.Store
	UserID    string
}

func Construct(config *peer.Configuration,
	data *data.Layer,
	consensus *consensus.Layer,
	gossip *gossip.Layer,
	hashedPublicKey [32]byte) *Layer {
	// Create the feed store.
	feedStore := feed.LoadStore(config.BlockchainStorage, config.BlockchainStorage.GetStore("metadata"))
	// Convert the byte array into a hex string.
	userID := hex.EncodeToString(hashedPublicKey[:])
	l := &Layer{
		consensus: consensus,
		data:      data,
		gossip:    gossip,
		Config:    config,
		FeedStore: feedStore,
		UserID:    userID,
	}
	// Register the registration consensus protocol.
	consensus.RegisterProtocol("registration", l.newRegistrationConsensusProtocol(config, gossip, l.FeedStore))
	// Load the registered users during the construction.
	l.loadRegisteredUsers(config.BlockchainStorage)
	return l
}

func (l *Layer) GetAddress() string {
	return l.consensus.GetAddress()
}

func (l *Layer) GetUserID() string {
	return l.UserID
}

func (l *Layer) Register() error {
	utils.PrintDebug("social", l.GetAddress(), "has initiated registration with id", l.UserID)
	regMetadata := content.CreateJoinMetadata(l.UserID, utils.Time())
	val := content.UnparseMetadata(regMetadata)
	paxosVal := types.PaxosValue{
		UniqID:      xid.New().String(),
		CustomValue: val,
	}
	_, err := l.consensus.ProposeWithProtocol("registration", paxosVal)
	if err != nil {
		return fmt.Errorf("error during registration: %v", err)
	}
	return nil
}

func (l *Layer) ProposeMetadata(metadata content.Metadata) (string, error) {
	utils.PrintDebug("social", l.GetAddress(), "is proposing a new post")
	val := content.UnparseMetadata(metadata)
	paxosVal := types.PaxosValue{
		UniqID:      xid.New().String(),
		CustomValue: val,
	}
	protocolID := feed.IDFromUserID(l.UserID)
	blockHash, err := l.consensus.ProposeWithProtocol(protocolID, paxosVal)
	if err != nil {
		return "", fmt.Errorf("could not propose metadata at social layer: %v", err)
	}
	return blockHash, nil
}

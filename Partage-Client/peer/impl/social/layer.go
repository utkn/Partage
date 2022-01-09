package social

import (
	"encoding/hex"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
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
	// Convert the byte array into a hex string.
	userID := hex.EncodeToString(hashedPublicKey[:])
	return &Layer{
		consensus: consensus,
		data:      data,
		gossip:    gossip,
		Config:    config,
		FeedStore: feed.LoadStore(),
		UserID:    userID,
	}
}

func (l *Layer) GetAddress() string {
	return l.consensus.GetAddress()
}

func (l *Layer) GetUserID() string {
	return l.UserID
}

func (l *Layer) Register() error {
	utils.PrintDebug("social", l.GetAddress(), "has initiated registration with id", l.UserID)
	newUserMsg := NewUserMessage{UserID: l.UserID}
	trspMsg, _ := l.Config.MessageRegistry.MarshalMessage(&newUserMsg)
	return l.gossip.Broadcast(trspMsg)
}

func (l *Layer) ProposeNewPost(info feed.PostInfo) error {
	utils.PrintDebug("social", l.UserID, "is proposing a new post")
	val := feed.MakeCustomPaxosValue(info)
	paxosVal := types.PaxosValue{
		UniqID:      xid.New().String(),
		CustomValue: val,
	}
	protocolID := feed.IDFromUserID(l.UserID)
	err := l.consensus.ProposeWithProtocol(protocolID, paxosVal)
	if err != nil {
		return err
	}
	return nil
}

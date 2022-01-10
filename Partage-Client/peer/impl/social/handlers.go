package social

import (
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(NewUserMessage{}, l.NewUserMessageHandler)
}

func (l *Layer) NewUserMessageHandler(msg types.Message, pkt transport.Packet) error {
	newUserMsg := msg.(*NewUserMessage)
	// For each user in the system, we maintain a separate consensus protocol instance.
	protocolID := feed.IDFromUserID(newUserMsg.UserID)
	alreadyExists := l.consensus.IsRegistered(protocolID)
	if !alreadyExists {
		utils.PrintDebug("social", l.GetAddress(), "is registering", newUserMsg.UserID)
		l.consensus.RegisterProtocol(protocolID, NewFeedConsensusProtocol(newUserMsg.UserID, l.Config, l.gossip, l.FeedStore))
		// Save the user id into the feed store as well.
		l.FeedStore.GetFeed(l.Config.BlockchainStorage, l.Config.BlockchainStorage.GetStore("metadata"), newUserMsg.UserID)
	}
	return nil
}

package social

import (
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(NewUserMessage{}, l.NewUserMessageHandler)
}

func (l *Layer) NewUserMessageHandler(msg types.Message, pkt transport.Packet) error {
	newUserMsg := msg.(*NewUserMessage)
	_, alreadyExists := l.FeedMap[newUserMsg.UserID]
	if !alreadyExists {
		l.FeedMap[newUserMsg.UserID] = NewFeed()
		// For each user in the system, we maintain a separate consensus protocol instance.
		protocolID := "feed-" + newUserMsg.UserID
		l.consensus.RegisterProtocol(protocolID, paxos.New(protocolID, l.Config, l.consensus.Gossip, FeedBlockFactory))
	}
	return nil
}

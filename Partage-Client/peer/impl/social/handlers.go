package social

import (
	"go.dedis.ch/cs438/peer/impl/consensus/protocol/paxos"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.Config.MessageRegistry.RegisterMessageCallback(NewUserMessage{}, l.NewUserMessageHandler)
}

func (l *Layer) NewUserMessageHandler(msg types.Message, pkt transport.Packet) error {
	newUserMsg := msg.(*NewUserMessage)
	// For each user in the system, we maintain a separate consensus protocol instance.
	protocolID := feed.FeedIDFromUserID(newUserMsg.UserID)
	alreadyExists := l.consensus.IsRegistered(protocolID)
	if !alreadyExists {
		l.consensus.RegisterProtocol(protocolID, paxos.New(protocolID, l.Config, l.consensus.Gossip,
			FeedBlockGenerator(newUserMsg.UserID),
			FeedBlockchainUpdater(l.FeedStore, newUserMsg.UserID),
			FeedProposalChecker(newUserMsg.UserID)))
	}
	return nil
}

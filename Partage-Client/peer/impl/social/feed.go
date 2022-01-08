package social

import (
	"errors"
	"go.dedis.ch/cs438/peer"
)

type FeedContent struct {
	Loaded    bool
	ContentID string
}

type Feed struct {
	Contents []FeedContent
}

func NewFeed() Feed {
	return Feed{}
}

func GetFeed(config *peer.Configuration, id string) (Feed, error) {
	feed := config.BlockchainStorage.GetStore(id)
	if feed == nil {
		return Feed{}, errors.New("feed not found")
	}
	return Feed{}, nil
}

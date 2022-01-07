package social

import (
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/peer/impl/data"
)

type Layer struct {
	consensus *consensus.Layer
	data      *data.Layer
}

func (l *Layer) GetAddress() string {
	return l.consensus.GetAddress()
}

func (l *Layer) PostContent(content string) {}

func (l *Layer) ReactToPost() {}

func (l *Layer) FollowUser() {}

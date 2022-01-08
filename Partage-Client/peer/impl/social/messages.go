package social

import (
	"go.dedis.ch/cs438/types"
)

type NewUserMessage struct {
	UserID string
}

func (n NewUserMessage) NewEmpty() types.Message {
	return &NewUserMessage{}
}

func (n NewUserMessage) Name() string {
	return "social-newuser"
}

func (n NewUserMessage) String() string {
	return "{social-newuser}"
}

func (n NewUserMessage) HTML() string {
	return "<>"
}

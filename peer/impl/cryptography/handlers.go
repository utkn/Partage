package cryptography

import (
	"fmt"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, l.PrivateMessageHandler)
}

func (l *Layer) PrivateMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", l.GetAddress(), "is at PrivateMessageHandler")
	privateMsg, ok := msg.(*types.PrivateMessage)
	if !ok {
		return fmt.Errorf("could not parse the private msg message")
	}
	// Process the embedded packet if we are in the recipient list.
	_, ok = privateMsg.Recipients[l.GetAddress()]
	if ok {
		transpPacket := transport.Packet{
			Header: pkt.Header,
			Msg:    privateMsg.Msg,
		}
		l.config.MessageRegistry.ProcessPacket(transpPacket.Copy())
		return nil
	}
	return nil
}
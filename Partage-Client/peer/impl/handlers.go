package impl

import (
	"encoding/json"
	"fmt"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func ParsePayload(payload json.RawMessage) (map[string]string, error) {
	payloadMap := make(map[string]string, 1)
	err := json.Unmarshal(payload, &payloadMap)
	if err != nil {
		return nil, err
	}
	return payloadMap, nil
}

func (n *node) ChatMessageHandler(msg types.Message, pkt transport.Packet) error {
	_, err := ParsePayload(pkt.Msg.Payload)
	if err != nil {
		return fmt.Errorf("could not handle chat message: %w", err)
	}
	//fmt.Println("Received message:", payload["Message"])
	return nil
}

func (n *node) EmptyMessageHandler(msg types.Message, pkt transport.Packet) error {
	return nil
}

func (n *node) PrivateMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", n.addr, "is at PrivateMessageHandler")
	privateMsg, ok := msg.(*types.PrivateMessage)
	if !ok {
		return fmt.Errorf("could not parse the private msg message")
	}
	// Process the embedded packet if we are in the recipient list.
	_, ok = privateMsg.Recipients[n.addr]
	if ok {
		transpPacket := transport.Packet{
			Header: pkt.Header,
			Msg:    privateMsg.Msg,
		}
		n.conf.MessageRegistry.ProcessPacket(transpPacket.Copy())
		return nil
	}
	return nil
}

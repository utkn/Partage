package cryptography

import (
	"encoding/json"
	"fmt"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, l.PrivatePostHandler)
}

func (l *Layer) PrivatePostHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", l.GetAddress(), "is at PrivatePostHandler")
	privateMsg, ok := msg.(*types.PrivatePost)
	if !ok {
		return fmt.Errorf("could not parse the private post message")
	}
	// Process the embedded packet if we are in the recipient list.
	ciphertext, ok := privateMsg.Recipients[l.GetAddress()]
	if !ok {
		return nil
	}
	//decrypt the encrypted AES key, using my RSA private key
	aesKey,err:=utils.DecryptWithPrivateKey(ciphertext,l.privateKey)
	if err!=nil{
		return err
	}
	//decrypt bytes using the AES symmetric key
	msgBytes,err:=utils.DecryptAES(privateMsg.Msg,aesKey) 
	if err!=nil{
		return err
	}
	//convert from bytes to transport.Message type
	var transportMsg transport.Message
    if err := json.Unmarshal(msgBytes, &transportMsg); err != nil {
        return err
    }
	
	//transport.Message
	transpPacket := transport.Packet{
		Header: pkt.Header,
		Msg:    &transportMsg,//privateMsg.Msg,
	}
	l.config.MessageRegistry.ProcessPacket(transpPacket.Copy())

	return nil
}
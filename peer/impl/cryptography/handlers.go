package cryptography

import (
	"encoding/json"
	"fmt"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(types.PrivatePost{}, l.PrivatePostHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchPKReplyMessage{}, l.SearchPKReplyMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchPKRequestMessage{}, l.SearchPKRequestMessageHandler)
}

func (l *Layer) PrivatePostHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", l.GetAddress(), "is at PrivatePostHandler")
	privateMsg, ok := msg.(*types.PrivatePost)
	if !ok {
		return fmt.Errorf("could not parse the private post message")
	}
	recipientsMap:= types.RecipientsMap{}
	if err:=recipientsMap.Decode(privateMsg.Recipients); err!=nil{
		//fmt.Println(err)
		return err
	}
	// Process the embedded packet if we are in the recipient list.
	ciphertext, ok := recipientsMap[l.socket.GetHashedPublicKey()]
	if !ok { //i'm not in the recipients list..
		return nil
	}
	fmt.Println(l.GetAddress(), " rcvd a private post for me!")
	//decrypt the encrypted AES key, using my RSA private key
	aesKey, err := utils.DecryptWithPrivateKey(ciphertext[:], l.GetPrivateKey())
	if err != nil {
		return err
	}

	//decrypt bytes using the AES symmetric key
	msgBytes, err := utils.DecryptAES(privateMsg.Msg, aesKey)
	if err != nil {
		return err
	}
	//convert from bytes to transport.Message type
	var transportMsg transport.Message

	if err := json.Unmarshal(msgBytes, &transportMsg); err != nil {
		return err
	}
	fmt.Println(l.GetAddress(), "rcvd the following private post:", transportMsg)
	//transport.Message
	transpPacket := transport.Packet{
		Header: pkt.Header,
		Msg:    &transportMsg, //privateMsg.Msg,
	}

	l.config.MessageRegistry.ProcessPacket(transpPacket.Copy())

	return nil
}

func (l *Layer) SearchPKReplyMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("searchPK", l.GetAddress(), "is at Search PK REPLY MessageHandler")
	searchPKReplyMsg, ok := msg.(*types.SearchPKReplyMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search PK reply msg")
	}
	CAPublicKey:=l.socket.GetCAPublicKey()
	if CAPublicKey==nil{
		fmt.Println("No CA-signed public key..")
		return nil
	}
	var signedPK types.SignedPublicKey
	err:=signedPK.Decode(searchPKReplyMsg.Response)

	// CHECK IF PUBLIC KEY IS SIGNED BY TRUSTED CA!
	isValid := err == nil && utils.VerifyPublicKeySignature(signedPK.PublicKey,signedPK.Signature,CAPublicKey)
	if isValid {
		l.notification.DispatchResponse(searchPKReplyMsg.RequestID, msg)
		
		bytesPK,_:=utils.PublicKeyToBytes(signedPK.PublicKey)
		//add entry to catalog
		l.socket.CatalogLock.Lock()
		l.socket.Catalog[utils.Hash(bytesPK)] = &signedPK
		l.socket.CatalogLock.Unlock()
	}

	return nil
}

func (l *Layer) SearchPKRequestMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("searchPK", l.GetAddress(), "is at Search PK REQUEST MessageHandler")
	searchPKRequestMsg, ok := msg.(*types.SearchPKRequestMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search PK request msg")
	}
	utils.PrintDebug("searchPK", l.GetAddress(), "looking for Public Key..")
	l.socket.CatalogLock.RLock()
	_, ok = l.processedSearchRequests[searchPKRequestMsg.RequestID]
	// Duplicate PK request received.
	if ok {
		l.socket.CatalogLock.RUnlock()
		utils.PrintDebug("searchPK", l.GetAddress(), "is ignoring the duplicate PK request.")
		return nil
	}
	l.socket.CatalogLock.RUnlock()
	// Save the request id.
	l.socket.CatalogLock.Lock()
	l.processedSearchRequests[searchPKRequestMsg.RequestID] = struct{}{}
	l.socket.CatalogLock.Unlock()
	neighbors := l.network.GetNeighbors()
	delete(neighbors, pkt.Header.RelayedBy)
	budgetMap := utils.DistributeBudget(searchPKRequestMsg.Budget-1, neighbors)
	// Forward with the correct budget.
	for neighbor, budget := range budgetMap {
		searchPKRequestMsg.Budget = budget
		transpMsg, _ := l.config.MessageRegistry.MarshalMessage(searchPKRequestMsg)
		_ = l.network.Route(searchPKRequestMsg.Origin, neighbor, neighbor, transpMsg)
	}
	//search for requested signed PK in my catalog
	var signedPK *types.SignedPublicKey
	//fmt.Println("searching for:",searchPKRequestMsg.Username,"\nmy:",l.socket.GetHashedPublicKey()," \n\nare equal?=",searchPKRequestMsg.Username == l.socket.GetHashedPublicKey())
	if searchPKRequestMsg.Username == l.socket.GetHashedPublicKey() {
		signedPK = l.socket.GetSignedPublicKey()
	} else {
		l.socket.CatalogLock.RLock()
		signedPK, ok = l.socket.Catalog[searchPKRequestMsg.Username]
		l.socket.CatalogLock.RUnlock()
		if !ok { //no signed PK found locally
			return nil //don't respond
		}
	}
	utils.PrintDebug("searchPK", l.GetAddress(), "FOUND THE Signed PK")
	bytes, err := signedPK.Encode()
	if err!=nil{
		fmt.Println("[ERROR] marshaling signed PK to send...")
		return nil
	}
	searchPKReplyMsg := types.SearchPKReplyMessage{
		RequestID: searchPKRequestMsg.RequestID,
		Response:  bytes,
	}
	transpMsg, _ := l.config.MessageRegistry.MarshalMessage(&searchPKReplyMsg)
	utils.PrintDebug("searchPK", l.GetAddress(), "is sending back a search PK reply to", pkt.Header.Source)
	_ = l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, searchPKRequestMsg.Origin, transpMsg)

	return nil
}

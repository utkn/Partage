package cryptography

import (
	"crypto/rsa"
	"crypto/x509"
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
	// Process the embedded packet if we are in the recipient list.
	ciphertext, ok := privateMsg.Recipients[l.username]
	if !ok { //i'm not in the recipients list..
		return nil
	}
	fmt.Println(l.GetAddress(), " rcvd a private post for me!")
	//decrypt the encrypted AES key, using my RSA private key
	aesKey,err:=utils.DecryptWithPrivateKey(ciphertext,l.myPrivateKey)
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
	fmt.Println(l.GetAddress(), "rcvd the following private post:",transportMsg)
	//transport.Message
	transpPacket := transport.Packet{
		Header: pkt.Header,
		Msg:    &transportMsg,//privateMsg.Msg,
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
	cert,err:=utils.PemToCertificate(searchPKReplyMsg.Response)
	//TODO: CHECK IF CERTIFICATE IS SIGNED BY TRUSTED CA! and if it belongs to the user it claims to belong (certificate check)
	isValid:= err==nil && cert!=nil &&  cert.Subject.Organization[0]==searchPKReplyMsg.Username//TODO: remove after implementing a validation check
	if isValid{
		_,ok:=cert.PublicKey.(*rsa.PublicKey) //check that certificate is using RSA
		if !ok{
			return nil
		} 
		l.notification.DispatchResponse(searchPKReplyMsg.RequestID, msg)

		//add certificate to catalog
		l.socket.CatalogLock.Lock()
		l.socket.Catalog[searchPKReplyMsg.Username]=cert
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
	utils.PrintDebug("searchPK", l.GetAddress(), "looking for",searchPKRequestMsg.Username," certificate..")
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
	//search for requested user's certificate in my catalog
	var cert *x509.Certificate
	if searchPKRequestMsg.Username==l.username{
		cert,_=x509.ParseCertificate(l.socket.GetCertificate().Certificate[0])
	}else{
		l.socket.CatalogLock.RLock()
		cert,ok=l.socket.Catalog[searchPKRequestMsg.Username]
		l.socket.CatalogLock.RUnlock()
		if !ok{ //no certificate found locally
			return nil //don't respond
		}
	}
	utils.PrintDebug("searchPK", l.GetAddress(),"FOUND THE CERTIFICATE for",searchPKRequestMsg.Username)
	byteCert,_:=utils.CertificateToPem(cert)
	searchPKReplyMsg := types.SearchPKReplyMessage{
		RequestID: searchPKRequestMsg.RequestID,
		Response: byteCert,
		Username: searchPKRequestMsg.Username,
	}
	transpMsg, _ := l.config.MessageRegistry.MarshalMessage(&searchPKReplyMsg)
	utils.PrintDebug("searchPK", l.GetAddress(), "is sending back a search PK reply to", pkt.Header.Source)
	_ = l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, searchPKRequestMsg.Origin, transpMsg)
	
	return nil
}
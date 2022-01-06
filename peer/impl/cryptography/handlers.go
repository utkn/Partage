package cryptography

import (
	"fmt"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchPKReplyMessage{}, l.SearchPKReplyMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchPKRequestMessage{}, l.SearchPKRequestMessageHandler)
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
	// CHECK IF PUBLIC KEY IS SIGNED BY TRUSTED CA!
	isValid := utils.VerifyPublicKeySignature(searchPKReplyMsg.Response.PublicKey,searchPKReplyMsg.Response.Signature,CAPublicKey)
	if isValid {
		l.notification.DispatchResponse(searchPKReplyMsg.RequestID, msg)
		
		bytesPK,_:=utils.PublicKeyToBytes(searchPKReplyMsg.Response.PublicKey)
		//add entry to catalog
		l.AddUserToCatalog(utils.Hash(bytesPK),&searchPKReplyMsg.Response)
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
	var signedPK *transport.SignedPublicKey
	//fmt.Println("searching for:",searchPKRequestMsg.Username,"\nmy:",l.socket.GetHashedPublicKey()," \n\nare equal?=",searchPKRequestMsg.Username == l.socket.GetHashedPublicKey())
	if searchPKRequestMsg.Username == l.socket.GetHashedPublicKey() {
		signedPK = l.socket.GetSignedPublicKey()
	} else {
		signedPK, ok = l.GetUserFromCatalog(searchPKRequestMsg.Username)
		if !ok { //no signed PK found locally
			return nil //don't respond
		}
	}
	utils.PrintDebug("searchPK", l.GetAddress(), "FOUND THE Signed PK")

	searchPKReplyMsg := types.SearchPKReplyMessage{
		RequestID: searchPKRequestMsg.RequestID,
		Response:  *signedPK,
	}
	transpMsg, _ := l.config.MessageRegistry.MarshalMessage(&searchPKReplyMsg)
	utils.PrintDebug("searchPK", l.GetAddress(), "is sending back a search PK reply to", pkt.Header.Source)
	_ = l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, searchPKRequestMsg.Origin, transpMsg) //TODO: no need to sign

	return nil
}

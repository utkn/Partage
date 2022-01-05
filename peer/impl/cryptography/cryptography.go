package cryptography

import (
	"crypto/rsa"

	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/tcptls"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	network                 *network.Layer
	gossip                  *gossip.Layer
	notification            *utils.AsyncNotificationHandler
	config                  *peer.Configuration
	socket                  *tcptls.Socket
	processedSearchRequests map[string]struct{}
	expandingConf           peer.ExpandingRing
}

func Construct(network *network.Layer, gossip *gossip.Layer, config *peer.Configuration) *Layer {
	socket, ok := config.Socket.(*tcptls.Socket)
	if !ok {
		panic("node must have a tcp socket in order to use tls")
	}
	_, ok = socket.GetCertificate().PrivateKey.(*rsa.PrivateKey)
	if !ok {
		panic("node must have a RSA based TLS Certificate")
	}
	return &Layer{
		network:                 network,
		gossip:                  gossip,
		config:                  config,
		socket:                  socket,
		processedSearchRequests: make(map[string]struct{}),
		notification:            utils.NewAsyncNotificationHandler(),
		expandingConf: peer.ExpandingRing{ //TODO: should be depend on the qnt of network nodes
			Initial: 1,
			Factor:  2,
			Retry:   5,
			Timeout: time.Second * 5,
		},
	}
}

func (l *Layer) GetAddress() string {
	return l.gossip.GetAddress()
}

//recipients will be a slice containing the each recipient hashed public key
func (l *Layer) SendPrivatePost(msg transport.Message, recipients [][32]byte) error {
	// Generate Symmetric Encryption Key (AES-256)
	aesKey, err := utils.GenerateAESKey()
	if err != nil {
		return err
	}
	// For each recipient, encrypt the aesKey with the user's RSA Public Key (associated with the user's TLS certificate)
	users := types.RecipientsMap{} //user_y:EncPK_x(aesKey),user_y:EncPK_y(aesKey),...
	var encryptedAESKey [128]byte 
	for _, username := range recipients {
		pk := l.SearchPublicKey(username, &l.expandingConf)
		if pk != nil {
			ciphertext, err := utils.EncryptWithPublicKey(aesKey, pk)
			if err != nil {
				return err
			}
			copy(encryptedAESKey[:],ciphertext)
			users[username] = encryptedAESKey
		}
	}

	// Encrypt Message with AES-256 key
	byteMsg, err := json.Marshal(msg) //from transport.Message to []byte
	if err != nil {
		return err
	}

	encryptedMsg, err := utils.EncryptAES(byteMsg, aesKey)
	if err != nil {
		return err
	}

	bytes,err:=users.Encode()
	if err!=nil{
		//fmt.Println(err)
		return err
	}
	//share Private Post
	privatePost := types.PrivatePost{
		Recipients: bytes,
		Msg:        encryptedMsg,
	}
	data, err := json.Marshal(&privatePost)
	if err != nil {
		return err
	}
	toSendMsg := transport.Message{
		Type:    privatePost.Name(),
		Payload: data,
	}
	fmt.Println(l.GetAddress()," Broadcasted privatePost to",len(users),"recipients!")
	err = l.gossip.Broadcast(toSendMsg) //share

	return err
}

func (l *Layer) GetPrivateKey() *rsa.PrivateKey {
	return l.socket.GetCertificate().PrivateKey.(*rsa.PrivateKey)
}

func (l *Layer) SearchPublicKey(hashedPK [32]byte, conf *peer.ExpandingRing) *rsa.PublicKey {
	// First look for a match locally.
	if hashedPK == l.socket.GetHashedPublicKey() {
		//x509Cert,_:=x509.ParseCertificate(l.socket.GetCertificate().Certificate[0])
		return &l.GetPrivateKey().PublicKey
	}
	l.socket.CatalogLock.RLock()
	signedPK, existsLocally := l.socket.Catalog[hashedPK]
	l.socket.CatalogLock.RUnlock()
	if existsLocally {
		return signedPK.PublicKey
	}

	// Initiate the expanding ring search.
	budget := conf.Initial
	for i := uint(0); i < conf.Retry; i++ {
		// Create the search request id.
		searchRequestID := xid.New().String()
		// Save the search request id.
		l.socket.CatalogLock.Lock()
		l.processedSearchRequests[searchRequestID] = struct{}{}
		l.socket.CatalogLock.Unlock()
		budgetMap := utils.DistributeBudget(budget, l.network.GetNeighbors())
		for neighbor, budget := range budgetMap {
			// Create the search request.
			msg := types.SearchPKRequestMessage{
				RequestID: searchRequestID,
				Origin:    l.GetAddress(),
				Username:  hashedPK,
				Budget:    budget,
			}
			transpMsg, _ := l.config.MessageRegistry.MarshalMessage(&msg)
			// Send the search request.
			utils.PrintDebug("searchPK", l.GetAddress(), "is unicasting a search PK request to", neighbor, "with expanding ring.")
			err := l.network.Unicast(neighbor, transpMsg)
			if err != nil {
				utils.PrintDebug("searchPK", "Could not unicast the search PK request")
				continue
			}
		}
		// Collect the received responses.
		collectedResponses := l.notification.MultiResponseCollector(searchRequestID, conf.Timeout, -1)
		utils.PrintDebug("searchPK", l.GetAddress(), "has received the following search PK RESPONSES during the timeout", collectedResponses)
		// Iterate through all the received responses within the timeout.
		var signedPK types.SignedPublicKey
		for _, resp := range collectedResponses {
			searchResp := resp.(*types.SearchPKReplyMessage)
			err:= signedPK.Decode(searchResp.Response)
			if err!=nil{
				fmt.Println("not able to unmarshal SignedPublicKey from search response")
				continue
			}
			//if it is in the collectedResponses it's because it is valid...
			return signedPK.PublicKey //found the user's Public Key
		}
		// no PK found yet..increase the budget and try again.
		budget = budget * conf.Factor
	}
	return nil
}




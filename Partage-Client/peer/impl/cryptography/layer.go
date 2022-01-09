package cryptography

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/tcptls"
	"go.dedis.ch/cs438/types"
)

type Layer struct {
	network *network.Layer

	notification            *utils.AsyncNotificationHandler
	config                  *peer.Configuration
	socket                  *tcptls.Socket
	processedSearchRequests map[string]struct{}
	expandingConf           peer.ExpandingRing
}

func Construct(network *network.Layer, config *peer.Configuration) *Layer {
	socket, ok := config.Socket.(*tcptls.Socket)
	if !ok {
		panic("node must have a tcp socket in order to use tls")
	}
	_, ok = socket.GetTLSCertificate().PrivateKey.(*rsa.PrivateKey)
	if !ok {
		panic("node must have a RSA based TLS Certificate")
	}
	return &Layer{
		network:                 network,
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
	return l.socket.GetAddress()
}

// Send encapsulates the transport.Message contained in pkt.Msg, into a types.SignedMessage by adding a layer of
//security and sends it through the TLS socket
func (l *Layer) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	// Add Validation check to packet's header (Signs packet with myPrivateKey!)
	pkt.AddValidation(l.GetPrivateKey(), l.GetSignedPublicKey())
	return l.network.Send(dest, pkt, timeout)
}

func (l *Layer) Unicast(dest string, msg transport.Message) error {
	table := l.network.GetRoutingTable()
	relay, ok := table[dest]
	// If the destination is a neighbor, it is the relay.
	if l.network.IsNeighbor(dest) {
		relay = dest
		ok = true
	}
	if !ok {
		utils.PrintDebug("communication", l.GetAddress(), "could not crypto unicast to", dest)
		return fmt.Errorf("could not find a relay for the unicast")
	}
	return l.Route(l.GetAddress(), relay, dest, msg)
}

// Only use cryptography.Route() when sending transport.Messages that actually need a cryptographic validation check
// added to packet's header!
func (l *Layer) Route(source string, relay string, dest string, msg transport.Message) error {
	header := transport.NewHeader(source, l.GetAddress(), dest, 0)
	pkt := transport.Packet{
		Header: &header,
		Msg:    &msg,
	}
	/*
		validation,err:=l.GenerateValidation(&msg)
		if err!=nil{
			return fmt.Errorf("error generating validation check : %w", err)
		}
		//set validation check for packet
		pkt.Header.Check=validation */
	err := l.Send(relay, pkt, time.Second*5)
	if err != nil {
		return fmt.Errorf("could not crypto route through socket: %w", err)
	}
	return nil
}

/*
func (l *Layer) GenerateValidation(msg *transport.Message) (*transport.Validation,error){
	byteMsg, err := json.Marshal(msg)
	if err!=nil{
		return nil,err
	}
	// Hash(packet.Msg||packet.Header.Source)
	hashedContent:=utils.Hash(append(byteMsg,[]byte(l.GetAddress())...))
	signature, err := rsa.SignPKCS1v15(rand.Reader, l.GetPrivateKey(), crypto.SHA256, hashedContent[:])
	if err != nil {
		return nil,err
	}

	return &transport.Validation{
		Signature: signature,
		SrcPublicKey: *l.socket.GetSignedPublicKey(),
	},nil
} */

func (l *Layer) GetPrivateKey() *rsa.PrivateKey {
	return l.socket.GetTLSCertificate().PrivateKey.(*rsa.PrivateKey)
}

func (l *Layer) GetUserFromCatalog(hashedPK [32]byte) (*transport.SignedPublicKey, bool) {
	l.socket.CatalogLock.RLock()
	defer l.socket.CatalogLock.RUnlock()
	signedPK, existsLocally := l.socket.Catalog[hashedPK]
	return signedPK, existsLocally
}

func (l *Layer) AddUserToCatalog(hashedPK [32]byte, sigPK *transport.SignedPublicKey) {
	l.socket.CatalogLock.Lock()
	defer l.socket.CatalogLock.Unlock()
	l.socket.Catalog[hashedPK] = sigPK
}

func (l *Layer) SearchPublicKey(hashedPK [32]byte, conf *peer.ExpandingRing) *rsa.PublicKey {
	// First look for a match locally.
	if hashedPK == l.socket.GetHashedPublicKey() {
		//x509Cert,_:=x509.ParseCertificate(l.socket.GetCertificate().Certificate[0])
		return &l.GetPrivateKey().PublicKey
	}

	signedPK, existsLocally := l.GetUserFromCatalog(hashedPK)
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
		//var signedPK types.SignedPublicKey
		for _, resp := range collectedResponses {
			searchResp := resp.(*types.SearchPKReplyMessage)
			//if it is in the collectedResponses it's because it is valid...
			return searchResp.Response.PublicKey // signedPK.PublicKey //found the user's Public Key
		}
		// no PK found yet..increase the budget and try again.
		budget = budget * conf.Factor
	}
	return nil
}

func (l *Layer) GetExpandingConf() *peer.ExpandingRing {
	return &l.expandingConf
}

func (l *Layer) GetHashedPublicKey() [32]byte {
	return l.socket.GetHashedPublicKey()
}

func (l *Layer) GetSignedPublicKey() *transport.SignedPublicKey {
	return l.socket.GetSignedPublicKey()
}

func (l *Layer) GetCAPublicKey() *rsa.PublicKey {
	return l.socket.GetCAPublicKey()
}

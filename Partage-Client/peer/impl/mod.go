package impl

import (
	"encoding/hex"
	"errors"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/social"
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"io"

	"regexp"
	"time"

	"go.dedis.ch/cs438/peer/impl/cryptography"
	"go.dedis.ch/cs438/peer/impl/data"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport/tcptls"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// node implements a peer to build a Peerster system
//
// - implements peer.Peer
type node struct {
	peer.Peer
	addr            string
	conf            peer.Configuration
	quitDistributor *utils.SignalDistributor
	quit            chan bool

	social    *social.Layer
	data      *data.Layer
	consensus *consensus.Layer
	gossip    *gossip.Layer
	network   *network.Layer
	// For tcp connections only
	cryptography *cryptography.Layer
}

// NewPeer creates a new peer.
func NewPeer(conf peer.Configuration) peer.Peer {
	// Create the quit signal channel.
	quitChannel := make(chan bool)
	// We wish to distribute the quit signal to multiple routines.
	quitDistributor := utils.NewSignalDistributor(quitChannel)
	quitDistributor.NewListener("server")

	tlsSock, isRunningTLS := conf.Socket.(*tcptls.Socket)
	if isRunningTLS {
		_ = tlsSock.RegisterUser()	
	} 
	
	// Create the layers.
	networkLayer := network.Construct(&conf)
	var cryptographyLayer *cryptography.Layer
	if isRunningTLS {
		cryptographyLayer = cryptography.Construct(networkLayer, &conf)
		cryptographyLayer.RegisterHandlers()
	}

	gossipLayer := gossip.Construct(networkLayer, cryptographyLayer, &conf, quitDistributor)
	consensusLayer := consensus.Construct(gossipLayer, &conf)
	dataLayer := data.Construct(gossipLayer, consensusLayer, networkLayer, &conf)
	var hashedPK [32]byte
	if isRunningTLS {
		hashedPK = cryptographyLayer.GetHashedPublicKey()
	}
	socialLayer := social.Construct(&conf, dataLayer, consensusLayer, gossipLayer, hashedPK)

	node := &node{
		addr: conf.Socket.GetAddress(),
		conf: conf,
		// We wish to distribute the quit signal to multiple routines.
		quitDistributor: quitDistributor,
		quit:            quitChannel,
		// Layers
		social:       socialLayer,
		data:         dataLayer,
		consensus:    consensusLayer,
		gossip:       gossipLayer,
		network:      networkLayer,
		cryptography: cryptographyLayer,
	}
	// Register the handlers.
	gossipLayer.RegisterHandlers()
	consensusLayer.RegisterHandlers()
	dataLayer.RegisterHandlers()
	socialLayer.RegisterHandlers() 

	conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, node.ChatMessageHandler)
	conf.MessageRegistry.RegisterMessageCallback(types.EmptyMessage{}, node.EmptyMessageHandler)
	conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, node.PrivateMessageHandler)
	// Start the quit signal distributor.
	go quitDistributor.SingleRun()

	return node // if node is not properly registered (has a valid signed certificate) he won't be able to participate in the network)
}

// Start implements peer.Service
func (n *node) Start() error {
	// Start the listener.
	if n.cryptography != nil {
		//TCP with TLS
		sock := n.conf.Socket.(*tcptls.Socket)
		go func() {
			for {
				// Accept incoming connections..
				tlsConn, keep, err := sock.Accept()
				if err != nil {
					if keep {
						continue
					}
					//socket closed..stopping node..
					fmt.Println("OUT 0")
					return
				} else {
					//create go routine to handle this connection (recv)
					go sock.HandleTLSConn(tlsConn, false)
				}
			}
		}()
		go func() {
			pktQueue := *sock.GetPktQueue()
			// Wait for new packets...
			for pkt := range pktQueue {
				if pkt == nil {
					fmt.Println("OUT 1")
					return
				}
				cpkt := pkt.Copy()
				utils.PrintDebug("network", n.addr, "listener has received a", cpkt.Msg.Type)
				table := n.GetRoutingTable()
				// Process the packet if the destination is this node.
				if cpkt.Header.Destination == n.addr {
					// Process the packet in a separate non-blocking goroutine.
					go func() {
						utils.PrintDebug("network", n.addr, "listener is about to process a", cpkt.Msg.Type)
						err := n.conf.MessageRegistry.ProcessPacket(cpkt)
						if err != nil {
							fmt.Printf("could not process the packet: %s", err.Error())
						}
					}()
					continue
				}
				// Try to route the packet otherwise.
				relay, ok := table[pkt.Header.Destination]
				if ok {
					pkt.Header.RelayedBy = n.addr
					utils.PrintDebug("network", n.addr, "is relaying", relay, "a", pkt.Msg.Type)
					err := n.network.Send(relay, *pkt, time.Second*1)
					if err != nil {
						fmt.Printf("could not relay the packet: %s", err.Error())
					}
				}
			}
		}()
	} else {
		go func() {
			quitListener, _ := n.quitDistributor.GetListener("server")
			for {
				select {
				case <-quitListener:
					return
				default:
					// Wait for a new packet.
					pkt, err := n.network.Receive(time.Second * 1)
					if errors.Is(err, transport.TimeoutErr(0)) {
						continue
					}
					if err != nil {
						fmt.Printf("could not receive from socket: %s", err.Error())
						return
					}
					utils.PrintDebug("network", n.addr, "listener has received a", pkt.Msg.Type)
					cpkt := pkt //pkt.Copy()
					table := n.GetRoutingTable()
					// Process the packet if the destination is this node.
					if cpkt.Header.Destination == n.addr {
						// Process the packet in a separate non-blocking goroutine.
						go func() {
							utils.PrintDebug("network", n.addr, "listener is about to process a", pkt.Msg.Type)
							err := n.conf.MessageRegistry.ProcessPacket(cpkt)
							if err != nil {
								fmt.Printf("could not process the packet: %s", err.Error())
							}
						}()
						continue
					}
					// Try to route the packet otherwise.
					relay, ok := table[cpkt.Header.Destination]
					if ok {
						cpkt.Header.RelayedBy = n.addr
						utils.PrintDebug("network", n.addr, "is relaying", relay, "a", pkt.Msg.Type)
						err := n.network.Send(relay, cpkt, time.Second*1)
						if err != nil {
							fmt.Printf("could not relay the packet: %s", err.Error())
						}
					}
				}
			}
		}()
	}

	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
	if n.cryptography != nil {
		//tcp with tls is being used
		sock := n.conf.Socket.(*tcptls.Socket)
		*sock.GetPktQueue() <- nil //exit queue reader goroutine
		sock.Close()
	}
	n.quit <- true
	return nil
}

// AddPeer implements peer.Messaging
func (n *node) AddPeer(addrs ...string) {
	n.network.AddPeer(addrs...)
}

// GetRoutingTable implements peer.Messaging
func (n *node) GetRoutingTable() peer.RoutingTable {
	return n.network.GetRoutingTable()
}

// SetRoutingEntry implements peer.Messaging
func (n *node) SetRoutingEntry(origin, relayAddr string) {
	n.network.SetRoutingEntry(origin, relayAddr)
}

// Unicast implements peer.Messaging
func (n *node) Unicast(dest string, msg transport.Message) error {
	if n.cryptography != nil {
		return n.cryptography.Unicast(dest, msg)
	}
	return n.network.Unicast(dest, msg)
}

// Broadcast implements peer.Messaging
func (n *node) Broadcast(msg transport.Message) error {
	return n.gossip.Broadcast(msg)
}

// Upload implements peer.DataSharing
func (n *node) Upload(data io.Reader) (metahash string, err error) {
	return n.data.Upload(data)
}

// Download implements peer.DataSharing
func (n *node) Download(metahash string) ([]byte, error) {
	return n.data.Download(metahash)
}

// Tag implements peer.DataSharing
func (n *node) Tag(name string, mh string) error {
	return n.data.Tag(name, mh)
}

// Resolve implements peer.DataSharing
func (n *node) Resolve(name string) string {
	return n.data.Resolve(name)
}

// GetCatalog implements peer.DataSharing
func (n *node) GetCatalog() peer.Catalog {
	return n.data.GetCatalog()
}

// UpdateCatalog implements peer.DataSharing
func (n *node) UpdateCatalog(key string, peer string) {
	n.data.UpdateCatalog(key, peer)
}

// SearchAll implements peer.DataSharing
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	return n.data.SearchAll(reg, budget, timeout)
}

// SearchFirst implements peer.DataSharing
func (n *node) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (string, error) {
	return n.data.SearchFirst(pattern, conf)
}

// TODO: Test this.
func (n *node) SharePostTest(info feed.PostInfo) error {
	return n.social.ProposeNewPost(info)
}

// RegisterUser implements peer.PartageClient
func (n *node) RegisterUser() error {
	return n.social.Register()
}

// SharePrivatePost implements peer.PartageClient
func (n *node) SharePrivatePost(msg transport.Message, recipients [][32]byte) error {
	//msg should be a marshaled types.Post message..
	return n.gossip.SendPrivatePost(msg, recipients)
}

// BlockUser implements peer.PartageClient
func (n *node) BlockUser(publicKeyHash [32]byte) {
	tlsSock, ok := n.conf.Socket.(*tcptls.Socket)
	if ok {
		tlsSock.Block(publicKeyHash)
	}
}

// UnblockUser implements peer.PartageClient
func (n *node) UnblockUser(publicKeyHash [32]byte) {
	tlsSock, ok := n.conf.Socket.(*tcptls.Socket)
	if ok {
		tlsSock.Unblock(publicKeyHash)
	}
}

// GetHashedPublicKey implements peer.PartageClient
func (n *node) GetHashedPublicKey() [32]byte {
	tlsSock, ok := n.conf.Socket.(*tcptls.Socket)
	if ok {
		return tlsSock.GetHashedPublicKey()
	}
	return [32]byte{}
}

// GetUserID implements peer.PartageClient
func (n *node) GetUserID() string {
	b := n.GetHashedPublicKey()
	return hex.EncodeToString(b[:])
}

// GetKnownUsers implements peer.PartageClient
func (n *node) GetKnownUsers() map[string]struct{} {
	return n.social.FeedStore.GetRegisteredUsers()
}

// GetSharedPosts implements peer.PartageClient
func (n *node) GetSharedPosts(userID string) []feed.FeedContent {
	return n.social.FeedStore.GetFeed(n.conf.BlockchainStorage, userID).GetContents()
}

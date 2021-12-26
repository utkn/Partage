package impl

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"go.dedis.ch/cs438/peer/impl/cryptography"
	"go.dedis.ch/cs438/peer/impl/data"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"

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
	gossip          *gossip.Layer
	data            *data.Layer
	network         *network.Layer
	consensus       *consensus.Layer
	cryptography	*cryptography.Layer
}

// NewPeer creates a new peer.
func NewPeer(conf peer.Configuration) peer.Peer {
	// Create the quit signal channel.
	quitChannel := make(chan bool)
	// We wish to distribute the quit signal to multiple routines.
	quitDistributor := utils.NewSignalDistributor(quitChannel)
	quitDistributor.NewListener("server")
	// Create the layers.
	networkLayer := network.Construct(&conf)
	cryptographyLayer := cryptography.Construct(networkLayer) //TODO:
	gossipLayer := gossip.Construct(cryptographyLayer, &conf, quitDistributor)
	consensusLayer := consensus.Construct(gossipLayer, &conf)
	dataLayer := data.Construct(gossipLayer, consensusLayer, networkLayer, &conf)
	node := &node{
		addr: conf.Socket.GetAddress(),
		conf: conf,
		// We wish to distribute the quit signal to multiple routines.
		quitDistributor: quitDistributor,
		quit:            quitChannel,
		// Layers
		network:   networkLayer,
		gossip:    gossipLayer,
		consensus: consensusLayer,
		data:      dataLayer,
		cryptography: cryptographyLayer,
	}
	// Register the handlers.
	gossipLayer.RegisterHandlers()
	consensusLayer.RegisterHandlers()
	dataLayer.RegisterHandlers()
	conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, node.ChatMessageHandler)
	conf.MessageRegistry.RegisterMessageCallback(types.EmptyMessage{}, node.EmptyMessageHandler)
	conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, node.PrivateMessageHandler)
	// Start the quit signal distributor.
	go quitDistributor.SingleRun()
	return node
}

// Start implements peer.Service
func (n *node) Start() error {
	// Start the listener.
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
				cpkt := pkt.Copy()
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
	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
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

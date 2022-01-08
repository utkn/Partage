package network

import (
	"fmt"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/utils"
	"sync"
	"time"

	"go.dedis.ch/cs438/transport"
)

type Layer struct {
	config       *peer.Configuration
	routingTable peer.RoutingTable
	tableLock    sync.Mutex
}

func Construct(config *peer.Configuration) *Layer {
	// Create the routing table.
	table := make(peer.RoutingTable, 1)
	// Get self address.
	addr := config.Socket.GetAddress()
	// Add self into the table.
	table[addr] = addr
	return &Layer{
		config:       config,
		routingTable: table,
	}
}

func (l *Layer) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	return l.config.Socket.Send(dest, pkt, timeout)
}

func (l *Layer) Receive(timeout time.Duration) (transport.Packet, error) {
	return l.config.Socket.Recv(timeout)
}

func (l *Layer) Route(source string, relay string, dest string, msg transport.Message) error {
	header := transport.NewHeader(source, l.GetAddress(), dest, 0)
	pkt := transport.Packet{
		Header: &header,
		Msg:    &msg,
	}
	utils.PrintDebug("network", l.GetAddress(), "is sending", relay, "a", pkt.Msg.Type)
	err := l.Send(relay, pkt, time.Second*1)
	if err != nil {
		return fmt.Errorf("could not route through socket: %w", err)
	}
	return nil
}

func (l *Layer) Unicast(dest string, msg transport.Message) error {
	utils.PrintDebug("communication", l.GetAddress(), "unicasting", msg.Type, "to", dest)
	table := l.GetRoutingTable()
	relay, ok := table[dest]
	// If the destination is a neighbor, it is the relay.
	if l.IsNeighbor(dest) {
		relay = dest
		ok = true
	}
	if !ok {
		utils.PrintDebug("communication", l.GetAddress(), "could not unicast to", dest)
		return fmt.Errorf("could not find a relay for the unicast")
	}
	return l.Route(l.GetAddress(), relay, dest, msg)
}

func (l *Layer) UnicastWithSource(source string, dest string, msg transport.Message) error {
	utils.PrintDebug("communication", l.GetAddress(), "unicasting", msg.Type, "to", dest, "with source", source)
	table := l.GetRoutingTable()
	relay, ok := table[dest]
	// If the destination is a neighbor, it is the relay.
	if l.IsNeighbor(dest) {
		relay = dest
		ok = true
	}
	if !ok {
		utils.PrintDebug("communication", l.GetAddress(), "could not unicast to", dest)
		return fmt.Errorf("could not find a relay for the unicast")
	}
	return l.Route(source, relay, dest, msg)
}

func (l *Layer) AddPeer(addrs ...string) {
	l.tableLock.Lock()
	defer l.tableLock.Unlock()
	for _, addr := range addrs {
		l.routingTable[addr] = addr
	}
}

func (l *Layer) GetRoutingTable() peer.RoutingTable {
	l.tableLock.Lock()
	defer l.tableLock.Unlock()
	cpy := make(peer.RoutingTable, len(l.routingTable))
	for k, v := range l.routingTable {
		cpy[k] = v
	}
	return cpy
}

// SetRoutingEntry implements peer.Messaging
func (l *Layer) SetRoutingEntry(origin, relayAddr string) {
	l.tableLock.Lock()
	defer l.tableLock.Unlock()
	// Delete the entry if the relay address is an empty string.
	if relayAddr == "" {
		delete(l.routingTable, origin)
		return
	}
	// Otherwise, put the entry into the table with overwrite.
	l.routingTable[origin] = relayAddr
}

func (l *Layer) GetAddress() string {
	return l.config.Socket.GetAddress()
}

func (l *Layer) GetNeighbors() map[string]struct{} {
	l.tableLock.Lock()
	defer l.tableLock.Unlock()
	// Create a set of neighbors.
	neighborSet := make(map[string]struct{}, len(l.routingTable))
	for _, relay := range l.routingTable {
		neighborSet[relay] = struct{}{}
	}
	delete(neighborSet, l.GetAddress())
	return neighborSet
}

func (l *Layer) ChooseRandomNeighbor(exclusionSet map[string]struct{}) (string, error) {
	neighborSet := l.GetNeighbors()
	// Delete this node.
	delete(neighborSet, l.GetAddress())
	neighbor, err := utils.ChooseRandom(neighborSet, exclusionSet)
	return neighbor, err
}

func (l *Layer) IsNeighbor(addr string) bool {
	l.tableLock.Lock()
	defer l.tableLock.Unlock()
	for _, neighbor := range l.routingTable {
		if neighbor == addr {
			return true
		}
	}
	return false
}

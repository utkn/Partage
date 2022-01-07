package udp

import (
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"go.dedis.ch/cs438/transport"
)

const bufSize = 65000

// NewUDP returns a new udp transport implementation.
func NewUDP() transport.Transport {
	return &UDP{}
}

// UDP implements a transport layer using UDP
//
// - implements transport.Transport
type UDP struct {
}

// CreateSocket implements transport.Transport
func (n *UDP) CreateSocket(address string) (transport.ClosableSocket, error) {
	// Resolve UDP address.
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	// Create the listening socket.
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	return &Socket{
		conn: conn,
		ins:  []transport.Packet{},
		outs: []transport.Packet{},
	}, nil
}

// Socket implements a network socket using UDP.
//
// - implements transport.Socket
// - implements transport.ClosableSocket
type Socket struct {
	insLock  sync.RWMutex
	outsLock sync.RWMutex
	conn     *net.UDPConn
	ins      []transport.Packet
	outs     []transport.Packet
}

// Close implements transport.Socket. It returns an error if already closed.
func (s *Socket) Close() error {
	return s.conn.Close()
}

// Send implements transport.Socket
func (s *Socket) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	pktBytes, err := pkt.Marshal()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("udp", dest, timeout)
	if err != nil {
		// Convert to a network error to specifically check for timeout errors.
		netErr, ok := err.(net.Error)
		if ok && netErr.Timeout() {
			return transport.TimeoutErr(0)
		}
		return err
	}
	defer conn.Close()
	_, err = conn.Write(pktBytes)
	if err == nil {
		s.outsLock.Lock()
		s.outs = append(s.outs, pkt.Copy())
		s.outsLock.Unlock()
	}
	return err
}

// Recv implements transport.Socket. It blocks until a packet is received, or
// the timeout is reached. In the case the timeout is reached, return a
// TimeoutErr.
func (s *Socket) Recv(timeout time.Duration) (transport.Packet, error) {
	deadline := time.Now().Add(timeout)
	err := s.conn.SetReadDeadline(deadline)
	if err != nil {
		return transport.Packet{}, err
	}
	buffer := make([]byte, bufSize)
	size, _, err := s.conn.ReadFromUDP(buffer)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return transport.Packet{}, transport.TimeoutErr(0)
	}
	if err != nil {
		return transport.Packet{}, err
	}
	var pkt transport.Packet
	err = pkt.Unmarshal(buffer[:size])
	if err != nil {
		return transport.Packet{}, err
	}
	s.insLock.Lock()
	s.ins = append(s.ins, pkt.Copy())
	s.insLock.Unlock()
	return pkt, nil
}

// GetAddress implements transport.Socket. It returns the address assigned. Can
// be useful in the case one provided a :0 address, which makes the system use a
// random free port.
func (s *Socket) GetAddress() string {
	addr := s.conn.LocalAddr().String()
	return addr
}

func copyPacketList(original []transport.Packet) []transport.Packet {
	cPkts := make([]transport.Packet, len(original))
	for i, pkt := range original {
		cPkts[i] = pkt.Copy()
	}
	return cPkts
}

// GetIns implements transport.Socket
func (s *Socket) GetIns() []transport.Packet {
	s.insLock.RLock()
	defer s.insLock.RUnlock()
	return copyPacketList(s.ins)
}

// GetOuts implements transport.Socket
func (s *Socket) GetOuts() []transport.Packet {
	s.outsLock.RLock()
	defer s.outsLock.RUnlock()
	return copyPacketList(s.outs)
}

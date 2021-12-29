package tcptls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
)

const bufSize = 65000

// NewTCP returns a new tcp transport implementation.
func NewTCP() transport.Transport {
	return &TCP{}
}

// TCP implements a transport layer using TCP
//
// - implements transport.Transport
type TCP struct {
}

// CreateSocket implements transport.Transport
func (n *TCP) CreateSocket(address string) (transport.ClosableSocket, error) {
	username := "user" + fmt.Sprint(time.Now().UnixNano()-int64(rand.Intn(100000)))
	// Load TLS certificate from memory or generate one (if no certificate is found)
	certificate, err := utils.LoadCertificate(false,username) //false for testing purposed, true if you want to store and load a certificate from persistent memory!
	if err != nil {
		return nil, err
	}
	// Create tls config with loaded certificate
	cfg := &tls.Config{Certificates: []tls.Certificate{*certificate}}
	// Create the listening TCP/TLS socket.
	listener, err := tls.Listen("tcp", address, cfg)
	if err != nil {
		return nil, err
	}

	return &Socket{
		listener:      &listener,
		ins:           []transport.Packet{},
		outs:          []transport.Packet{},
		tlsConfig:     &tls.Config{Certificates: []tls.Certificate{*certificate}, InsecureSkipVerify: true}, //TODO: beware the insecureskipverify (=true means it will not check if the certificate issuer is trusted)!!
		myCertificate: certificate,
		Catalog:       make(map[string]*x509.Certificate),
		username:      username,
	}, nil
}

// Socket implements a network socket using TCP.
//
// - implements transport.Socket
// - implements transport.ClosableSocket
type Socket struct {
	insLock       sync.RWMutex
	outsLock      sync.RWMutex
	listener      *net.Listener
	ins           []transport.Packet
	outs          []transport.Packet
	tlsConfig     *tls.Config
	myCertificate *tls.Certificate
	Catalog       map[string]*x509.Certificate
	CatalogLock   sync.RWMutex
	username      string
}

// Close implements transport.Socket. It returns an error if already closed.
func (s *Socket) Close() error {
	return (*s.listener).Close()
}

// Send implements transport.Socket
func (s *Socket) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	pktBytes, err := pkt.Marshal()
	if err != nil {
		return err
	}
	// Use Dialer to allow timeout on dial call
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", dest, s.tlsConfig)
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
	if err != nil {
		return err
	}

	s.outsLock.Lock()
	s.outs = append(s.outs, pkt.Copy())
	s.outsLock.Unlock()

	go func() { //TODO: may follow other approach later if this adds to much overhead to socket..
		//Save neighbors Cert..
		cert := conn.ConnectionState().PeerCertificates[0]
		username := cert.Subject.Organization[0]
		s.CatalogLock.RLock()
		_, exist := s.Catalog[username]
		s.CatalogLock.RUnlock()
		if !exist {
			s.CatalogLock.Lock()
			s.Catalog[username] = cert
			s.CatalogLock.Unlock()
		}
	}()

	return nil
}

// Recv implements transport.Socket. It blocks until a packet is received, or
// the timeout is reached. In the case the timeout is reached, return a
// TimeoutErr.
func (s *Socket) Recv(timeout time.Duration) (transport.Packet, error) {
	conn, err := (*s.listener).Accept()
	if err != nil {
		return transport.Packet{}, err
	}
	tlscon := conn.(*tls.Conn)
	defer tlscon.Close()
	deadline := time.Now().Add(timeout)
	err = tlscon.SetReadDeadline(deadline)
	if err != nil {
		return transport.Packet{}, err
	}
	buffer := make([]byte, bufSize)
	size, err := tlscon.Read(buffer)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return transport.Packet{}, transport.TimeoutErr(0)
	}
	if err != nil {
		return transport.Packet{}, err
	}
	var pkt transport.Packet
	err = pkt.Unmarshal(buffer[:size])
	if err != nil {
		//try to read the rest of the packet
		//Note..the pkts holding a certificate are too large so we're not able to read it in one Read() call, this is how we can go arround that problem
		cum:=append([]byte{}, buffer[:size]...)
		for{
			size, err := tlscon.Read(buffer)
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return transport.Packet{}, transport.TimeoutErr(0)
			}
			if err!=nil{
				return transport.Packet{}, err
			}
			cum=append(cum, buffer[:size]...)
			err = pkt.Unmarshal(cum)
			if err!=nil{
				continue
			}else{
				break
			}
		}
		//return transport.Packet{}, err
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
	return (*s.listener).Addr().String()
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

func (s *Socket) GetCertificate() *tls.Certificate {
	return s.myCertificate
}

func (s *Socket) GetUsername() string {
	return s.username
}

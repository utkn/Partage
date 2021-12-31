package tcptls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
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
		pktQueue: make(chan *transport.Packet,1024), 
		connPool: newConnPool(),

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
	connPool ConnPool
	pktQueue chan *transport.Packet
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

	newConn:=false
	conn:=s.connPool.GetConn(dest)
	if conn==nil{
		newConn=true
		// Use Dialer to allow timeout on dial call
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", dest, s.tlsConfig)
		if err != nil {
			// Convert to a network error to specifically check for timeout errors.
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				return transport.TimeoutErr(0)
			}
			return err
		}
		// Add conn to pool
		s.connPool.AddConn(dest,conn)

		// Create a pkt listening goroutine for this new conn
		go s.HandleTLSConn(conn)
	}

	_, err = conn.Write(pktBytes)
	if err != nil {
		//conn may have been closed due to read time out...
		//return s.Send(dest,pkt,timeout)
		return err
	}

	s.outsLock.Lock()
	s.outs = append(s.outs, pkt.Copy())
	s.outsLock.Unlock()

	if newConn{
		// Save neighbors Cert..
		go func() { //TODO: may follow other approach later if this adds to much overhead to socket..
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
	}

	return nil
}

func (s *Socket) Accept() (*tls.Conn,error){
	conn, err := (*s.listener).Accept()
	if err != nil {
		return nil,err
	}
	tlsConn,ok:= conn.(*tls.Conn)
	if !ok{
		return nil,err
	}
	//add to connPool
	s.connPool.AddConn(tlsConn.RemoteAddr().String(),tlsConn)

	return tlsConn,nil
}

// Recv implements transport.Socket. It blocks until a packet is received, or
// the timeout is reached. In the case the timeout is reached, return a
// TimeoutErr.
func (s *Socket) Recv(timeout time.Duration) (transport.Packet, error) {
	// Method is kinda useless for the TCP socket since,
	// packets are received through accepted/open connections, 
	// and not directly through the socket

	//create go routine with HandleTLSConn instead (to recv packets from TCP Conn)
	return transport.Packet{}, nil
}

func (s *Socket) AddIn(pkt *transport.Packet){
	s.insLock.Lock()
	defer s.insLock.Unlock()
	s.ins = append(s.ins, pkt.Copy())
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

func (s *Socket) RemoveConn(tlsConn *tls.Conn){
	s.connPool.CloseConn(tlsConn.RemoteAddr().String())
}

func (s *Socket) GetPktQueue() *chan *transport.Packet{
	return &s.pktQueue
}

func (s *Socket) HandleTLSConn(tlsConn *tls.Conn){
	fmt.Println(s.GetAddress()," created a goroutine to handle ",tlsConn.RemoteAddr().String()," conn")
	recvTimeout:=time.Second * 60 * 2 //2 minutes
	//conn
	var pkt transport.Packet
	buffer := make([]byte, bufSize)
	deadline := time.Now().Add(recvTimeout)
	err := tlsConn.SetReadDeadline(deadline)
	if err != nil {
		//discard conn
		s.RemoveConn(tlsConn)
	}
	for{
		size, err:= tlsConn.Read(buffer)
		if err != nil {
			s.RemoveConn(tlsConn)
			return 
		}
		/*if errors.Is(err, os.ErrDeadlineExceeded){
			//end conn
			s.RemoveConn(tlsConn)
			return 
		}
		if errors.Is(err, io.EOF){
			//conn closed on the other end
			s.RemoveConn(tlsConn)
			return 
		}
		if err != nil {
			fmt.Printf("could not receive from socket: %s", err.Error())
			return 
		}*/
		//s.m.Lock()
		err = pkt.Unmarshal(buffer[:size])
		if err != nil { //unmarshaling error
			//try to read the rest of the packet
			//Note..the pkts holding a certificate are too large so we're not able to read it in one Read() call, this is how we can go arround that problem
			cum:=append([]byte{}, buffer[:size]...)
			for{
				size, err := tlsConn.Read(buffer)
				if err != nil {
					s.RemoveConn(tlsConn)
					return 
				}
				/*
				if errors.Is(err, os.ErrDeadlineExceeded) {
					//end conn
					s.RemoveConn(tlsConn)
					return 
				}
				if errors.Is(err, io.EOF){
					//conn closed on the other end
					s.RemoveConn(tlsConn)
					return 
				}
				if err!=nil{
					fmt.Printf("could not receive from socket: %s", err.Error())
					s.RemoveConn(tlsConn)
					return 
					//continue
				}*/
				cum=append(cum, buffer[:size]...)
				err = pkt.Unmarshal(cum)
				if err!=nil{
					continue
				}else{
					break
				}
			}
		}
		s.AddIn(&pkt)

		//send to packet queue
		cpkt:=pkt.Copy()
		s.pktQueue<-&cpkt
	}
}

func (s *Socket) CloseConns(){
	s.connPool.Close()
}
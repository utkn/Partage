package tcptls

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
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
	// Load my TLS certificate from memory and my public key signature or generate one (if no certificate is found)
	certificate, err := utils.LoadCertificate(false) //false for testing purposed, true if you want to store and load a certificate from persistent memory!
	if err != nil {
		return nil, err
	}
	// Create tls config with loaded certificate
	cfg := &tls.Config{Certificates: []tls.Certificate{*certificate}, InsecureSkipVerify: true, ClientAuth: tls.RequestClientCert}
	// Create the listening TCP/TLS socket.
	listener, err := tls.Listen("tcp", address, cfg)
	if err != nil {
		return nil, err
	}

	ca := utils.LoadCACertificate()
	pkSignature := utils.LoadPublicKeySignature()

	var blockedUsers map[[32]byte]struct{}
	if utils.TESTING {
		blockedUsers = make(map[[32]byte]struct{})
	} else {
		blockedUsers, _ = utils.LoadBlockedUsers()
	}
	fp, _ := utils.OpenFileToAppend(utils.BlockedUsersPath) //to save blocked users in persistent memory

	return &Socket{
		listener:         &listener,
		ins:              []transport.Packet{},
		outs:             []transport.Packet{},
		tlsConfig:        cfg,
		myTLSCertificate: certificate,
		CA:               ca,
		Catalog:          make(map[[32]byte]*transport.SignedPublicKey), //hashed public key maps to *rsa.PublicKey
		myPKSignature:    pkSignature,
		pktQueue:         make(chan *transport.Packet, 1024),
		connPool:         newConnPool(),
		blockedUsers:     blockedUsers,
		blockedIPs:       make(map[string][32]byte), //to reject rumors by origin!
		fpBlockedUsers:   fp,
	}, nil
}

// Socket implements a network socket using TCP.
//
// - implements transport.Socket
// - implements transport.ClosableSocket
type Socket struct {
	insLock          sync.RWMutex
	outsLock         sync.RWMutex
	listener         *net.Listener
	ins              []transport.Packet
	outs             []transport.Packet
	tlsConfig        *tls.Config
	myTLSCertificate *tls.Certificate
	myPKSignature    []byte
	Catalog          map[[32]byte]*transport.SignedPublicKey
	CatalogLock      sync.RWMutex
	connPool         ConnPool
	pktQueue         chan *transport.Packet
	CA               *x509.Certificate
	//blocking mechanism
	blockedUsers      map[[32]byte]struct{}
	blockedUsersMutex sync.RWMutex
	blockedIPs        map[string][32]byte
	blockedIPsMutex   sync.RWMutex
	fpBlockedUsers    *os.File
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

	//newConn := false
	conn := s.connPool.GetConn(dest)
	if conn == nil {
		//newConn = true
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
		s.connPool.AddConn(dest, conn)
		utils.PrintDebug("tls", s.GetAddress(), " created a goroutine to handle ", dest, " conn")

		// Create a pkt listening goroutine for this new conn
		go s.HandleTLSConn(conn, true)
	}

	_, err = conn.Write(pktBytes)
	if err != nil {
		//conn may have been closed due to read time out...
		//return s.Send(dest,pkt,timeout)
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
		s.connPool.AddConn(dest, conn)
		utils.PrintDebug("tls", s.GetAddress(), " created a goroutine to handle ", dest, " conn")

		// Create a pkt listening goroutine for this new conn
		go s.HandleTLSConn(conn, true)
		
		//return err
	}

	s.outsLock.Lock()
	s.outs = append(s.outs, pkt.Copy())
	s.outsLock.Unlock()

	/*
		if newConn {
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
		} */

	return nil
}

func (s *Socket) Accept() (*tls.Conn, bool, error) {
	conn, err := (*s.listener).Accept()
	if err != nil {
		return nil, false, err
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, true, errors.New("not tls conn")
	}

	tlsConn.Handshake()
	if tlsConn.ConnectionState().HandshakeComplete && tlsConn.ConnectionState().PeerCertificates[0].CheckSignatureFrom(s.CA) == nil {
		return tlsConn, true, nil
	}
	fmt.Println("Refused Connection: Certificate is not signed by the trusted CA!")
	return nil, true, errors.New("refused: certificate isnt signed by trusted CA")
}

func (s *Socket) HandleTLSConn(tlsConn *tls.Conn, connSaved bool) {
	recvTimeout := time.Second * 60 * 3 //3 minutes
	//conn
	var pkt transport.Packet
	buffer := make([]byte, bufSize)
	deadline := time.Now().Add(recvTimeout)
	err := tlsConn.SetReadDeadline(deadline)
	if err != nil {
		//discard conn
		s.RemoveConn(tlsConn)
	}

	for {
		size, err := tlsConn.Read(buffer)
		if err != nil {
			s.RemoveConn(tlsConn)
			return
		}

		err = pkt.Unmarshal(buffer[:size])
		if err != nil { //unmarshaling error
			//try to read the rest of the packet
			//Note..the pkts holding a certificate are too large so we're not able to read it in one Read() call, this is how we can go arround that problem
			cum := append([]byte{}, buffer[:size]...)
			for {
				size, err := tlsConn.Read(buffer)
				if err != nil {
					s.RemoveConn(tlsConn)
					return
				}
				cum = append(cum, buffer[:size]...)
				err = pkt.Unmarshal(cum)
				if err != nil {
					continue
				} else {
					break
				}
			}
		}

		//check packet RelayedBy parameter and not the actual tlsConn source addr parameter. Because you can't use the same addr socket to listen from and to dial from, so nodes will dial from a different addr than the one they are listening to. We just care about the listening-socket addr
		//add to connPool
		if !connSaved {
			if !s.connPool.ConnExists(pkt.Header.RelayedBy) {
				s.connPool.AddConn(pkt.Header.RelayedBy, tlsConn)
				utils.PrintDebug("tls", s.GetAddress(), " created a goroutine to handle ", pkt.Header.RelayedBy, " conn")
			}
			connSaved = true
		}

		//VALIDATE PACKET!
		// Validate packet signatures
		if pkt.Validate(s.GetCAPublicKey()) != nil {
			//signatures aren't valid..drop packet
			fmt.Println("pkt signature no valid")
			continue
		}
		// Check for banned users packets and drop the ones that are for me! (still relay packets from blocked users)
		if pkt.Header.Destination == s.GetAddress() && pkt.Header.Check != nil {
			pkBytes, _ := utils.PublicKeyToBytes(pkt.Header.Check.SrcPublicKey.PublicKey)
			if s.IsBlocked(utils.Hash(pkBytes)) {
				fmt.Println("avoided packet from blocked user")
				continue
			}
		}
		/*If node A blocks node B, node A still forwards node B’s packets but stops
		processing/reading node B’s messages or updating his view with node B’s view
		(i.e., node A stops	saving node B’s rumors)*/

		s.AddIn(&pkt)

		//send to packet queue
		cpkt := pkt.Copy()
		s.pktQueue <- &cpkt
	}
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

func (s *Socket) AddIn(pkt *transport.Packet) {
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

func (s *Socket) GetTLSCertificate() *tls.Certificate {
	return s.myTLSCertificate
}

func (s *Socket) RemoveConn(tlsConn *tls.Conn) {
	s.connPool.CloseConn(tlsConn.RemoteAddr().String())
}

func (s *Socket) GetPktQueue() *chan *transport.Packet {
	return &s.pktQueue
}

func (s *Socket) CloseSocketConnections() {
	s.connPool.Close()
}

func (s *Socket) UpdateCertificate(cert *x509.Certificate, privKey *rsa.PrivateKey, publicKeySignature []byte) error {
	keyPem, err := utils.PrivateKeyToPem(privKey)
	if err != nil {
		return err
	}
	certPem, err := utils.CertificateToPem(cert)
	if err != nil {
		return err
	}
	tlsCert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return err
	}
	var newTLSConf *tls.Config
	ca := utils.LoadCACertificate()
	if ca != nil {
		//TLS will check every connection against his CA's Certificate
		caCertPool := x509.NewCertPool()
		caPem, _ := utils.CertificateToPem(ca)
		caCertPool.AppendCertsFromPEM(caPem)
		//TODO:
		newTLSConf = &tls.Config{Certificates: []tls.Certificate{tlsCert}, InsecureSkipVerify: true, RootCAs: caCertPool, ClientCAs: caCertPool, ClientAuth: tls.RequireAndVerifyClientCert}
	} else {
		newTLSConf = &tls.Config{Certificates: []tls.Certificate{tlsCert}, InsecureSkipVerify: true, ClientAuth: tls.RequestClientCert}
	}
	addr := s.GetAddress()
	s.Close()
	newListener, err := tls.Listen("tcp", addr, newTLSConf)
	if err != nil {
		return err
	}

	s.tlsConfig = newTLSConf
	s.listener = &newListener
	s.myTLSCertificate = &tlsCert
	s.connPool = newConnPool()
	s.CA = ca
	s.myPKSignature = publicKeySignature

	return nil
}

func (s *Socket) GetTLSConfig() *tls.Config {
	return s.tlsConfig
}

func (s *Socket) GetPublicKey() *rsa.PublicKey {
	return &s.myTLSCertificate.PrivateKey.(*rsa.PrivateKey).PublicKey
}

func (s *Socket) GetSignedPublicKey() *transport.SignedPublicKey {
	return &transport.SignedPublicKey{PublicKey: s.GetPublicKey(), Signature: s.myPKSignature}
}

func (s *Socket) GetHashedPublicKey() [32]byte {
	//stringify user's public key
	bytes, err := utils.PublicKeyToBytes(s.GetPublicKey())
	if err == nil {
		return utils.Hash(bytes)
	}
	return [32]byte{}
}

func (s *Socket) GetCAPublicKey() *rsa.PublicKey {
	if s.CA != nil {
		pubK, ok := s.CA.PublicKey.(*rsa.PublicKey)
		if ok {
			return pubK
		}
	}
	return nil
}

func (s *Socket) IsBlocked(publicKeyHash [32]byte) bool {
	s.blockedUsersMutex.RLock()
	defer s.blockedUsersMutex.RUnlock()
	_, exists := s.blockedUsers[publicKeyHash]
	return exists
}

func (s *Socket) Block(publicKeyHash [32]byte) {
	s.blockedUsersMutex.Lock()
	defer s.blockedUsersMutex.Unlock()
	s.blockedUsers[publicKeyHash] = struct{}{}
	utils.AppendToFile(publicKeyHash[:], s.fpBlockedUsers)
}

func (s *Socket) Unblock(publicKeyHash [32]byte) {
	s.blockedUsersMutex.Lock()
	delete(s.blockedUsers, publicKeyHash)
	s.blockedUsersMutex.Unlock()
	//remove blocked ip associated with user's pk
	s.blockedIPsMutex.Lock()
	for k, v := range s.blockedIPs {
		if v == publicKeyHash {
			delete(s.blockedIPs, k)
		}
	}
	s.blockedIPsMutex.Unlock()
	s.storeBlockedUsers() //re-write blocked-users.db file with updated blocked users
}

func (s *Socket) IsBlockedIP(addr string) bool {
	s.blockedIPsMutex.RLock()
	defer s.blockedIPsMutex.RUnlock()
	_, exists := s.blockedIPs[addr]
	return exists
}

func (s *Socket) HasBlockedIPs() bool {
	s.blockedIPsMutex.RLock()
	defer s.blockedIPsMutex.RUnlock()
	return len(s.blockedIPs) > 0
}

func (s *Socket) AddBlockedIP(addr string, publicKeyHash [32]byte) {
	s.blockedIPsMutex.Lock()
	defer s.blockedIPsMutex.Unlock()
	s.blockedIPs[addr] = publicKeyHash
}

func (s *Socket) storeBlockedUsers() error {
	s.blockedUsersMutex.Lock()
	defer s.blockedUsersMutex.Unlock()
	s.fpBlockedUsers.Close()
	fp, err := utils.OpenFileToWrite(utils.BlockedUsersPath)
	if err != nil {
		return err
	}
	s.fpBlockedUsers = fp
	for k := range s.blockedUsers {
		utils.AppendToFile(k[:], s.fpBlockedUsers)
	}
	s.fpBlockedUsers.Close()
	s.fpBlockedUsers, err = utils.OpenFileToAppend(utils.BlockedUsersPath)
	if err != nil {
		return err
	}
	return nil
}

func (s *Socket) GetBlockedIPs() []string {
	s.blockedIPsMutex.RLock()
	defer s.blockedIPsMutex.RUnlock()
	ips := make([]string, len(s.blockedIPs))
	i := 0
	for ip := range s.blockedIPs {
		ips[i] = ip
		i++
	}
	return ips
}

package impl

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"partage-ca/server"
	"sync"
	"time"
)

type certificateAuthority struct {
	listener      net.Listener
	//storage
	usersCatalog   map[[32]byte]struct{}
	catalogMutex  sync.RWMutex
	
	myCertificate *x509.Certificate
	myPrivateKey  *rsa.PrivateKey
	fp *os.File
}

func NewServer() server.Server {
	//init TLS socket listener
	l, cert, sk, err := initTLSSocket(server.Addr)
	if err != nil {
		fmt.Println("[ERROR] creating TLS socket listener...", err)
		return nil
	}
	//load taken usernames from persistent memory
	keys,err := LoadUsers()
	if err != nil {
		fmt.Println("[ERROR] reading from Public Keys file...", err)
		return nil
	}
	fp,err:= OpenFileToAppend()
	if err != nil {
		fmt.Println("[ERROR] opening Public Keys file in append mode...", err)
		return nil
	}

	s := &certificateAuthority{
		listener:      l,
		usersCatalog:   keys,
		myCertificate: cert,
		myPrivateKey:  sk,
		fp: fp,
	}

	return s
}

func (s *certificateAuthority) Start() error {
	fmt.Println("listening on "+s.GetAddress()+" ...")
	go func() {
		for {
			//Accept incoming connections
			conn, err := s.listener.Accept()
			if err != nil {
				//socket closed..
				return
			} else {
				tlsConn := conn.(*tls.Conn)
				go s.handleTLSConn(tlsConn)
			}
		}
	}()
	return nil
}

func (s *certificateAuthority) Stop() error {
	fmt.Println("\nsmoothly stopping server...")
	s.listener.Close()
	s.fp.Close()
	return nil
}

func (s *certificateAuthority) GetAddress() string {
	return s.listener.Addr().String()
}

func (s *certificateAuthority) handleTLSConn(conn *tls.Conn) {
	defer conn.Close()

	conn.Handshake() //since no Read/Write is being called we need to do this explicity

	writeTimeout := time.Second * 3
	deadline := time.Now().Add(writeTimeout)
	err := conn.SetWriteDeadline(deadline)
	if err != nil {
		fmt.Println("[ERROR] setting write timeout for conn...", err)
		return
	}

	//client sends his certificate (to-be signed)
	//by accepting a tls conn we are able to access the clients TLS self-signed certificate
	if len(conn.ConnectionState().PeerCertificates)==0{
		fmt.Println("[ERROR] there is some problem with the client's TLS certificate",conn.ConnectionState())
		return 
	}
	clientCert := conn.ConnectionState().PeerCertificates[0]
	clientPublicKey, ok := clientCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		fmt.Println("[ERROR] client's certificate isn't using RSA")
		return
	}
	_,clientPublicKeyBytes,err:= PublicKeyToString(clientPublicKey)
	if err!=nil {
		fmt.Println("[ERROR] converting RSA public key to string")
		return
	}
	clientPublicKeyHash:=Hash(clientPublicKeyBytes)
	fmt.Println("handling registration request ...")

	// Check if public key is not taken
	s.catalogMutex.RLock()
	if _, exists := s.usersCatalog[clientPublicKeyHash]; exists {
		s.catalogMutex.RUnlock()
		// Send msg to client saying that public key is already taken
		msg := &server.Message{Type: "ERROR", Payload: []byte("public key is taken")}
		msgBytes, err := msg.Encode()
		if err != nil {
			fmt.Println("[ERROR] marshaling {public key is taken} msg to send...", err)
			return
		}
		_, err = conn.Write(msgBytes)
		if err != nil {
			fmt.Println("[ERROR] sending err message...", err)
		}
		return //...
	}
	s.catalogMutex.RUnlock()

	s.catalogMutex.Lock()
	// Sign client's certificate with CA!
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCert, s.myCertificate, clientPublicKey, s.myPrivateKey)
	if err != nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] signing certificate...", err)
		return
	}
	clientCertPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes})
	if clientCertPem == nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] encoding certificate to PEM...", err)
		return
	}
	// Sign client's public key with CA's private key
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.myPrivateKey, crypto.SHA256, clientPublicKeyHash[:])
	if err != nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] signing public key...", err)
		return
	}

	// Record public key as taken
	err = AppendToFile(clientPublicKeyHash[:],s.fp)
	if err != nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] storing public key in persistent-memory...", err)
		return
	}
	s.usersCatalog[clientPublicKeyHash] = struct{}{}
	s.catalogMutex.Unlock()

	// Prepare registration message to send (SignedCertificate,PublicKeySignature)
	registration:=&server.Registration{
		SignedCertificate: clientCertPem,
		PublicKeySignature: signature,
	}
	payload,err:=registration.Encode()
	if err!=nil{
		fmt.Println("[ERROR] marshaling payload to json...", err)
		return
	}
	msg := &server.Message{Type: "OK", Payload: payload}
	msgBytes, err := msg.Encode()
	if err != nil {
		fmt.Println("[ERROR] marshaling msg to json...", err)
		return
	}
	// Send signed certificate to client
	_, err = conn.Write(msgBytes)
	if err != nil {
		fmt.Println("[ERROR] writing to TLS connection...", err)
		return
	}
	fmt.Println("[REGISTER] user is now registered!")

	return
}

func initTLSSocket(address string) (net.Listener, *x509.Certificate, *rsa.PrivateKey, error) {
	//load CA certificate from memory or generate one (if no certificate is found)
	certificate, err := LoadCertificate(true)
	if err != nil {
		return nil, nil, nil, err
	}
	// Create tls config with loaded certificate
	cfg := &tls.Config{Certificates: []tls.Certificate{*certificate}, ClientAuth: tls.RequestClientCert, InsecureSkipVerify: true}

	// Create the listening TCP/TLS socket.
	listener, err := tls.Listen("tcp", address, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	x509Cert, _ := x509.ParseCertificate(certificate.Certificate[0])
	sk := certificate.PrivateKey.(*rsa.PrivateKey)
	return listener, x509Cert, sk, nil
}

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
	"net/mail"
	"net/smtp"
	"os"
	"partage-ca/server"
	"strconv"
	"sync"
	"time"
)

type certificateAuthority struct {
	listener      net.Listener
	smtpAuth 	*smtp.Auth
	//storage
	usersCatalog   map[[32]byte]struct{}
	emailsCatalog map[string]struct{}
	catalogMutex  sync.RWMutex
	
	myCertificate *x509.Certificate
	myPrivateKey  *rsa.PrivateKey
	fpPublicKeys *os.File
	fpEmails *os.File
}

func NewServer() server.Server {
	//init TLS socket listener
	l, cert, sk, auth,err := initTLSSocket(server.Addr)
	if err != nil {
		fmt.Println("[ERROR] creating TLS socket listener...", err)
		return nil
	}
	//load taken usernames from persistent memory
	keys,emails,err := LoadUsers()
	if err != nil {
		fmt.Println("[ERROR] reading from Public Keys file...", err)
		return nil
	}
	fp,err:= OpenFileToAppend(server.UsersPath)
	if err != nil {
		fmt.Println("[ERROR] opening Public Keys file in append mode...", err)
		return nil
	}
	fpEmails,err:= OpenFileToAppend(server.EmailsPath)
	if err != nil {
		fmt.Println("[ERROR] opening Public Keys file in append mode...", err)
		return nil
	}

	s := &certificateAuthority{
		listener:      l,
		smtpAuth: auth,
		usersCatalog:   keys,
		emailsCatalog: emails,
		myCertificate: cert,
		myPrivateKey:  sk,
		fpPublicKeys: fp,
		fpEmails: fpEmails,
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
	s.fpPublicKeys.Close()
	s.fpEmails.Close()
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

	// Get e-mail address from user's certificate
	if len(clientCert.Subject.Organization)==0{
		// Send msg to client saying that 
		msg := &server.Message{Type: "ERROR", Payload: []byte("empty e-mail address field")}
		msgBytes, err := msg.Encode()
		if err != nil {
			fmt.Println("[ERROR] marshaling {no email address} msg to send...", err)
			return
		}
		_, err = conn.Write(msgBytes)
		if err != nil {
			fmt.Println("[ERROR] sending err message...", err)
		}
		return 
	}
	clientEmail:=clientCert.Subject.Organization[0]
	if _,err:=mail.ParseAddress(clientEmail);err!=nil{
		//invalid e-mail address
		msg := &server.Message{Type: "ERROR", Payload: []byte("invalid e-mail address")}
		msgBytes, err := msg.Encode()
		if err != nil {
			fmt.Println("[ERROR] marshaling {no email address} msg to send...", err)
			return
		}
		_, err = conn.Write(msgBytes)
		if err != nil {
			fmt.Println("[ERROR] sending err message...", err)
		}
		return 
	}
	
	s.catalogMutex.RLock()	
	// Check if public key is not taken
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
	// Check if email is not taken
	if _, exists := s.emailsCatalog[clientEmail]; exists {
		s.catalogMutex.RUnlock()
		// Send msg to client saying that public key is already taken
		msg := &server.Message{Type: "ERROR", Payload: []byte("e-mail address is taken")}
		msgBytes, err := msg.Encode()
		if err != nil {
			fmt.Println("[ERROR] marshaling {email is taken} msg to send...", err)
			return
		}
		_, err = conn.Write(msgBytes)
		if err != nil {
			fmt.Println("[ERROR] sending err message...", err)
		}
		return //...
	}
	s.catalogMutex.RUnlock()
	// To avoid people registering with the same e-mail while CA is waiting for Verification Code
	s.catalogMutex.Lock()
	s.usersCatalog[clientPublicKeyHash] = struct{}{}
	s.emailsCatalog[clientEmail] = struct{}{}
	s.catalogMutex.Unlock()
	fmt.Println("sending verification code to",clientEmail,"...")
	// Tell user to check e-mail inbox
	msg := &server.Message{Type: "WARNING", Payload: []byte("Check your "+clientEmail+" inbox for the Verification Code (valid for 4 minutes)")}
	msgBytes, err := msg.Encode()
	if err != nil {
		fmt.Println("[ERROR] marshaling {check inbox} msg to send...", err)
		return
	}
	_, err = conn.Write(msgBytes)
	if err != nil {
		fmt.Println("[ERROR] sending {check inbox} message...", err)
		return
	}

	// Generate and Send Verification Code to user's e-mail (waits 4 minutes for verification code)
	if err:=s.VerifyUser(clientEmail,conn); err!=nil{
		//unable to verify
		//close connection
		conn.Close()
		s.catalogMutex.Lock()
		delete(s.usersCatalog,clientPublicKeyHash)
		delete(s.emailsCatalog,clientEmail)
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] on receiving Verification Code from",clientEmail,"--->",err)
		return 
	}
	fmt.Println("User e-mail was verified!")
	s.catalogMutex.Lock()
	// Sign client's certificate with CA!
	clientCert.Subject.Organization=nil //remove e-mail from user's certificate
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
	err = AppendToFile(clientPublicKeyHash[:],s.fpPublicKeys)
	if err != nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] storing public key in persistent-memory...", err)
		return
	}
	// Record e-mail as taken
	err = AppendToFile([]byte(clientEmail+"\n"),s.fpEmails)
	if err != nil {
		s.catalogMutex.Unlock()
		fmt.Println("[ERROR] storing e-mail in persistent-memory...", err)
		return
	}
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
	msg = &server.Message{Type: "OK", Payload: payload}
	msgBytes, err = msg.Encode()
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

func initTLSSocket(address string) (net.Listener, *x509.Certificate, *rsa.PrivateKey, *smtp.Auth,error) {
	//load CA certificate from memory or generate one (if no certificate is found)
	certificate, err := LoadCertificate(true)
	if err != nil {
		return nil, nil, nil,nil, err
	}
	// Create tls config with loaded certificate
	cfg := &tls.Config{Certificates: []tls.Certificate{*certificate}, ClientAuth: tls.RequestClientCert, InsecureSkipVerify: true}

	// Create the listening TCP/TLS socket.
	listener, err := tls.Listen("tcp", address, cfg)
	if err != nil {
		return nil, nil, nil, nil,err
	}
	x509Cert, _ := x509.ParseCertificate(certificate.Certificate[0])
	sk := certificate.PrivateKey.(*rsa.PrivateKey)
	//smtp server auth
	auth:=smtp.PlainAuth("",server.SmtpUsername,server.SmtpPassword,server.SmtpHost)
	return listener, x509Cert, sk,&auth,nil
}

func (s *certificateAuthority) SendVerificationCode(email string) (string,error){
	var challenge string
	if server.TESTING{
		challenge=strconv.Itoa(12348765)
	}else{
		challenge=strconv.Itoa(GenerateChallenge())	
	}
	msg := []byte("From: "+server.SmtpUsername+"\r\n" +
        "To: "+email+"\r\n" +
        "Subject: Partage Verification Code\r\n\r\n" +
        "Welcome to Partage! Here you have your Verification Code: "+challenge+"\r\n")
	err := smtp.SendMail(server.SmtpHost+":"+server.SmtpPort,*s.smtpAuth,server.SmtpUsername,[]string{email},msg)
	return challenge,err
} 

func (s *certificateAuthority) VerifyUser(email string,conn *tls.Conn) error{
	challenge,err:=s.SendVerificationCode(email)
	if err!=nil{
		return err
	}

	deadline := time.Now().Add(time.Second*60*4) //4 minutes timeout
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return err
	}
	buf:=make([]byte,4096)
	size, err := conn.Read(buf)
	if err != nil {
		return err 
	}
	var msg server.Message 
	err = msg.Decode(buf[:size])
	if err != nil {
		return err 
	}
	responseChallenge:=string(msg.Payload)
	if challenge!=responseChallenge{
		fmt.Println("rcvd:",responseChallenge," | expected:",challenge)
		return fmt.Errorf("received invalid verification code")
	}

	return nil //success!
}

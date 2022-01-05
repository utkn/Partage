package tcptls

import (
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)
const serverAddr = "127.0.0.1:1234" //Certificate Authority Server address

// Since we are using Certificates in order to authenticate users, the users authentication is directly related to the TLS Socket
func (tlsSock *Socket) RegisterUser() error {
	bufSize := 65000
	buf := make([]byte, bufSize)
	var msg types.CertificateAuthorityMessage
	// Dial CA Server using my current TLS certificate (CA server will interpret it as a sign-request)
	dialTimeout := 5 * time.Second
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: dialTimeout}, "tcp", serverAddr, tlsSock.GetTLSConfig())
	if err != nil {
		// Convert to a network error to specifically check for timeout errors.
		netErr, ok := err.(net.Error)
		if ok && netErr.Timeout() {
			fmt.Println("[ERROR] timeout while trying to establish connection with CA server...")
			return transport.TimeoutErr(0)
		}
		if err==io.EOF{
			fmt.Println("[WARNING] the CA server closed the connection!")
		}else{
			fmt.Println("[ERROR] dialing CA server...", err)
		}
		return err
	}
	deadline := time.Now().Add(10 * time.Second) // Waits for CA-server response for..10 seconds
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		fmt.Println("[ERROR] setting read timeout with CA server...", err)
		return err
	}
	size, err := conn.Read(buf)
	if err != nil {
		fmt.Println("CA server didn't respond...", err)
		return err
	}
	err = msg.Decode(buf[:size])
	if err != nil { //unmarshaling error
		//try to read the rest of the packet
		//Note..the pkts holding a certificate are too large so we're not able to read it in one Read() call, this is how we can go arround that problem
		cum := append([]byte{}, buf[:size]...)
		for {
			size, err := conn.Read(buf)
			if err != nil {
				fmt.Println("[ERROR] reading from CA server...", err)
				return err
			}
			cum = append(cum, buf[:size]...)
			err = msg.Decode(cum)
			if err != nil {
				continue
			} else {
				break
			}
		}
	}
	
	//PROCESS CA response
	if msg.Type == "ERROR" {
		fmt.Println("[ERROR] " + string(msg.Payload))
		return errors.New(string(msg.Payload))
	} else if msg.Type == "OK" {
		var details types.Registration
		//SUCESS!
		err = details.Decode(msg.Payload)
		if err!=nil{
			fmt.Println("[ERROR] marshaling Registration details from CA server OK-response...", err)
			return err
		}
		newCert, err := utils.PemToCertificate(details.SignedCertificate)
		if err != nil {
			fmt.Println("[ERROR] converting signed PEM certificate to x509.Certificate...", err)
			return err
		}
		// Store signed-certificate
		_, err = utils.StoreCertificate(newCert)
		if err != nil {
			fmt.Println("[ERROR] storing signed certificate...", err)
			return err
		}
		// Store CA's certificate
		_, err = utils.StoreCACertificate(conn.ConnectionState().PeerCertificates[0])
		if err != nil {
			fmt.Println("[ERROR] storing CA's certificate...", err)
			return err
		}
		// Store CA's signature of my Public Key
		err = utils.StorePublicKeySignature(details.PublicKeySignature)
		if err != nil {
			fmt.Println("[ERROR] storing public key signature...", err)
			return err
		}

		// Update all certificate-dependent attributes with this newly-signed certificate
		tlsSock.UpdateCertificate(newCert, tlsSock.GetCertificate().PrivateKey.(*rsa.PrivateKey), details.PublicKeySignature)
	} else {
		fmt.Println("[ERROR] unknown type of CA message:", msg.Type)
		return errors.New("unknown type of CA msg")
	}

	return nil
}

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
	//check if Certificate in use is self-signed, if so..
	utils.PrintDebug("tls", "DEBUG: registering new user...")
	if res, _ := utils.TLSIsSelfSigned(tlsSock.GetTLSCertificate()); !res {
		utils.PrintDebug("tls", "user already registered")
		return nil
	}
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
			utils.PrintDebug("tls", "[ERROR] timeout while trying to establish connection with CA server...")
			return transport.TimeoutErr(0)
		}
		if err == io.EOF {
			utils.PrintDebug("tls", "[WARNING] the CA server closed the connection!")
		} else {
			utils.PrintDebug("tls", "[ERROR] dialing CA server...", err)
		}
		return err
	}

	deadline := time.Now().Add(15 * time.Second) // Waits for CA-server response for..15 seconds
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		utils.PrintDebug("tls", "[ERROR] setting read timeout with CA server...", err)
		return err
	}
	size, err := conn.Read(buf)
	if err != nil {
		utils.PrintDebug("tls", "CA server didn't respond...try again later!", err)
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
				utils.PrintDebug("tls", "[ERROR] reading from CA server...", err)
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
		utils.PrintDebug("tls", "[ERROR] "+string(msg.Payload))
		return errors.New(string(msg.Payload))
	} else if msg.Type == "WARNING" {
		utils.PrintDebug("tls", string(msg.Payload))
		var input string
		if utils.TESTING {
			time.Sleep(time.Second * 2)
			input = "12348765"
		} else {
			fmt.Print("[VERIFICATION CODE]: ")
			//Read from stdin
			fmt.Scanln(&input)
		}
		utils.PrintDebug("tls", "Sending "+input+" code!")
		codeMsg := &types.CertificateAuthorityMessage{
			Type:    "CODE",
			Payload: []byte(input),
		}
		bytes, _ := codeMsg.Encode()
		conn.Write(bytes)
	} else {
		utils.PrintDebug("tls", "[ERROR] unknown type of CA message:", msg.Type)
		return errors.New("unknown type of CA msg")
	}

	deadline = time.Now().Add(15 * time.Second) // Waits for CA-server response for..15 seconds
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		utils.PrintDebug("tls", "[ERROR] setting read timeout with CA server...", err)
		return err
	}
	//WAIT FOR CA RESPONSE
	size, err = conn.Read(buf)
	if err != nil {
		utils.PrintDebug("tls", "CA server didn't respond...try again later!", err)
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
				utils.PrintDebug("tls", "[ERROR] reading from CA server...", err)
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
		utils.PrintDebug("tls", "[ERROR] "+string(msg.Payload))
		return errors.New(string(msg.Payload))
	} else if msg.Type == "OK" {
		var details types.Registration
		//SUCESS!
		err = details.Decode(msg.Payload)
		if err != nil {
			utils.PrintDebug("tls", "[ERROR] marshaling Registration details from CA server OK-response...", err)
			return err
		}
		newCert, err := utils.PemToCertificate(details.SignedCertificate)
		if err != nil {
			utils.PrintDebug("tls", "[ERROR] converting signed PEM certificate to x509.Certificate...", err)
			return err
		}
		// Store signed-certificate
		_, err = utils.StoreCertificate(newCert)
		if err != nil {
			utils.PrintDebug("tls", "[ERROR] storing signed certificate...", err)
			return err
		}
		// Store CA's certificate
		_, err = utils.StoreCACertificate(conn.ConnectionState().PeerCertificates[0])
		if err != nil {
			utils.PrintDebug("tls", "[ERROR] storing CA's certificate...", err)
			return err
		}
		// Store CA's signature of my Public Key
		err = utils.StorePublicKeySignature(details.PublicKeySignature)
		if err != nil {
			utils.PrintDebug("tls", "[ERROR] storing public key signature...", err)
			return err
		}

		// Update all certificate-dependent attributes with this newly-signed certificate
		tlsSock.UpdateCertificate(newCert, tlsSock.GetTLSCertificate().PrivateKey.(*rsa.PrivateKey), details.PublicKeySignature)
	} else {
		utils.PrintDebug("tls", "[ERROR] unknown type of CA message:", msg.Type)
		return errors.New("unknown type of CA msg")
	}

	utils.PrintDebug("tls", "DEBUG: successfully registered user!")
	return nil
}

package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"time"
)

const dir = "partage-storage/crypto/"
const certificatePath = dir + "cert.pem"
const keyPath = dir + "key.pem"


func LoadCertificate() (*tls.Certificate,error) {

	cert,err:=tls.LoadX509KeyPair(certificatePath, keyPath)
	if err!=nil{
		//generate a new key-pair
		privateKey, _ := generateKeyPair()
		keyPem,err:=storeKeyPair(privateKey, keyPath)
		if err!=nil{
			return nil,err
		}
		//generate a new certificate from the newly-generated key-pair
		certificate, _:= generateCertificate(privateKey,nil) //SELF-SIGNED! TODO: ..to later be implemented with the CA
		certPem,err:=storeCertificate(certificate,certificatePath)
		if err!=nil{
			return nil,err
		}
		cert, err = tls.X509KeyPair(certPem, keyPem)
		if err!=nil{
			return nil,err
		}
	}
	return &cert,err
}

/*
func LoadCryptoData() (*ecdsa.PrivateKey, *x509.Certificate) {

	privateKey, err0 := loadKeyPair(keyPath)
	certificate,err1:= loadCertificate(certificatePath)

	if err0 != nil || err1!=nil{ //no key file OR no certificate file
		//generate a new key-pair
		privateKey, _ = generateKeyPair()
		storeKeyPair(privateKey, keyPath)
		//generate a new certificate from the newly-generated key-pair
		certificate, _= generateCertificate(privateKey,nil) //SELF-SIGNED! TODO: ..to later be implemented with the CA
		storeCertificate(certificate,certificatePath)
	}

	return privateKey,certificate
}


func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("inexistent certificate file at " + path)
	}
	return x509.ParseCertificate(data)
}

func loadKeyPair(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("inexistent key file at " + path)
	}
	privKey, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return nil, err
	}
	return privKey.(*ecdsa.PrivateKey), nil
}
*/

func storeCertificate(cert *x509.Certificate, path string) ([]byte,error) {
	//serialize generated certificate into file
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if pemCert == nil {
		return nil,errors.New("failed to encode certificate to PEM")
	}
	if err := os.WriteFile(path, pemCert, 0644); err != nil {
		return nil,err
	}
	return pemCert,nil
}

func storeKeyPair(privateKey *ecdsa.PrivateKey, path string) ([]byte,error) {
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil,errors.New("unable to marshal key pair")
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	if pemKey == nil {
		return nil,errors.New("failed to encode key to PEM")
	}
	if err := os.WriteFile("key.pem", pemKey, 0600); err != nil {
		return nil,err
	}
	return privBytes,nil
}

//used to generate a private and public key using the P-256 elliptic curve
func generateKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

//used to generate a signed certificate (if signingAuthority==nil, certificate is self-signed)
//returns new certificate as ASN.1 DER data (can be parsed to x509.Certificate object with x509.ParseCertificate(der []byte) function)
func generateCertificate(privateKey *ecdsa.PrivateKey, signingAuthority *x509.Certificate) (*x509.Certificate, error) {
	//each certificate needs a unique serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Partage"},
		},
		//DNSNames:  []string{"localhost"}, //to make the certificate only valid for the localhost domain
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * 365 * 24 * time.Hour), //TODO: time validity of the certificate (currently valid for 1 year)

		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	//create certificate from the template
	var certBytes []byte
	if signingAuthority != nil {
		//signed by authority
		certBytes,err= x509.CreateCertificate(rand.Reader, &template, signingAuthority, &privateKey.PublicKey, privateKey)
	} else {
		//self-signed
		certBytes,err= x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
		
	}
	if err!=nil{
		return nil,err
	}
	return x509.ParseCertificate(certBytes)
}

package impl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"partage-ca/server"
	"strconv"
	"strings"
	"time"
)


func LoadCertificate(fromPersistentMem bool) (*tls.Certificate, error) {
	if fromPersistentMem {
		wd, _ := os.Getwd()
		rt := wd[:strings.Index(wd, "Partage-CA")]

		cert, err := tls.LoadX509KeyPair(rt+server.CertificatePath, rt+server.KeyPath)
		if err != nil {

			//generate a new RSA key-pair 
			privateKey, _ := generateKey()

			keyPem, err := storePrivateKey(privateKey, rt+server.KeyPath)
			if err != nil {
				return nil, err
			}

			//generate a new certificate from the newly-generated key-pair
			certificate, _ := generateCertificate(privateKey,nil) //SELF-SIGNED! TODO: ..to later be implemented with the CA

			certPem, err := storeCertificate(certificate, rt+server.CertificatePath)
			if err != nil {
				return nil, err
			}

			cert, err = tls.X509KeyPair(certPem, keyPem)
			if err != nil {
				return nil, err
			}
			return &cert, nil
		}
		return &cert, nil
	} else {
		// Generate a new fresh self-signed certificate..(GOOD FOR TESTING PURPOSES!)
		privateKey, _ := generateKey()
		keyPem, err := privateKeyToPem(privateKey)
		if err != nil {
			return nil, err
		}

		certificate, _ := generateCertificate(privateKey, nil)
		certPem, err := certificateToPem(certificate)
		if err != nil {
			return nil, err
		}

		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, err
		}

		return &cert, nil
	}
}


func privateKeyToPem(privateKey *rsa.PrivateKey) ([]byte, error) {
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	if pemKey == nil {
		return nil, errors.New("failed to encode key to PEM")
	}

	return pemKey, nil
}

func certificateToPem(cert *x509.Certificate) ([]byte, error) {
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if pemCert == nil {
		return nil, errors.New("failed to encode certificate to PEM")
	}
	return pemCert, nil
}

func storeCertificate(cert *x509.Certificate, path string) ([]byte, error) {
	//serialize generated certificate into file
	pemCert, err := certificateToPem(cert)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, pemCert, 0644); err != nil {
		return nil, err
	}

	return pemCert, nil
}

func storePrivateKey(privateKey *rsa.PrivateKey, path string) ([]byte, error) {
	pemKey, err := privateKeyToPem(privateKey)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, pemKey, 0600); err != nil {
		return nil, err
	}

	return pemKey, nil
}


//used to generate a private and public key using the P-256 elliptic curve
func generateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 1024)
}

//used to generate a signed certificate (if signingAuthority==nil, certificate is self-signed)
//returns new certificate as ASN.1 DER data (can be parsed to x509.Certificate object with x509.ParseCertificate(der []byte) function)
func generateCertificate(privateKey *rsa.PrivateKey,signingAuthority *x509.Certificate) (*x509.Certificate, error) {
	//each certificate needs a unique serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PARTAGE-CA"},
			Country: []string{"CH"},
		},
		//DNSNames:  []string{"localhost"}, //to make the certificate only valid for the localhost domain
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * 365 * 24 * time.Hour), //TODO: time validity of the certificate (currently valid for 1 year)
		IsCA: true,
		KeyUsage: x509.KeyUsageCertSign|x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	//create certificate from the template
	var certBytes []byte
	if signingAuthority != nil {
		//signed by authority
		certBytes, err = x509.CreateCertificate(rand.Reader, &template, signingAuthority, &privateKey.PublicKey, privateKey)
	} else {
		//self-signed
		certBytes, err = x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)

	}
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certBytes)
}

func PublicKeyToPem(publicKey *rsa.PublicKey) ([]byte, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, errors.New("failed to marshal public key")
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubBytes})
	if pemKey == nil {
		return nil, errors.New("failed to encode key to PEM")
	}

	return pemKey, nil
}

func PublicKeyToString(publicKey *rsa.PublicKey) (string,[]byte,error){
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "",nil, errors.New("failed to marshal public key")
	}
	return string(pubBytes),pubBytes,nil
}

func storePublicKey(k *rsa.PublicKey,path string) (error){
	pemKey, err := PublicKeyToPem(k)
	if err != nil {
		return  err
	}
	if err := os.WriteFile(path, pemKey, 0600); err != nil {
		return err
	}
	return nil
}

func StorePublicKey(k *rsa.PublicKey,i int) (error){
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	filename:="user"+strconv.Itoa(i)+".pem"
	return storePublicKey(k,rt+server.UsersPath+filename)
}

func loadPublicKey(path string) *rsa.PublicKey{
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pubKey, err := x509.ParsePKCS1PublicKey(data)
	if err != nil {
		return nil
	}
	return pubKey
}

func Hash(bytes []byte) [32]byte{
	return sha256.Sum256(bytes)
}

/*
func loadPublicKeysFromDirectory(dir string) (map[string]struct{},error) {
	// read certificate files
	keyFiles, err := filepath.Glob(filepath.Join(dir, "*.pem"))
	if err != nil {
		return nil,err
	}
	keys := make(map[string]struct{})
	for _, file := range keyFiles {
		key := loadPublicKey(file)
		if key != nil {
			str,_,err:=PublicKeyToString(key)
			if err!=nil{
				continue
			}
			keys[str]=struct{}{} //add entry
		}
	}
	return keys, nil
} 

func LoadPublicKeysFromDirectory() (map[string]struct{},error){
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	return loadPublicKeysFromDirectory(rt+server.UsersPath)
}
*/

package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"io"

	//"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"

	"math/big"
	"os"
	"strings"
	"time"
)

const dir = "Partage/partage-storage/crypto/"
const certificatePath = dir + "cert.pem"
const keyPath = dir + "key.pem"

func LoadCertificate(fromPersistentMem bool,username string) (*tls.Certificate, error) {
	if fromPersistentMem {
		wd, _ := os.Getwd()
		rt := wd[:strings.Index(wd, "Partage")]

		cert, err := tls.LoadX509KeyPair(rt+certificatePath, rt+keyPath)
		if err != nil {

			//generate a new key-pair
			privateKey, _ := generateKey()

			keyPem, err := storeKey(privateKey, rt+keyPath)
			if err != nil {
				return nil, err
			}

			//generate a new certificate from the newly-generated key-pair
			certificate, _ := generateCertificate(privateKey, username,nil) //SELF-SIGNED! TODO: ..to later be implemented with the CA

			certPem, err := storeCertificate(certificate, rt+certificatePath)
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

		certificate, _ := generateCertificate(privateKey,username, nil)
		certPem, err := CertificateToPem(certificate)
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

// EncryptWithPublicKey encrypts data with public key
func EncryptWithPublicKey(msg []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	//ciphertext, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, publicKey, msg, nil)
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, msg)
	if err != nil {
		return nil, err
	} 
	return ciphertext, nil
}

// DecryptWithPrivateKey decrypts data with private key
func DecryptWithPrivateKey(ciphertext []byte, priv *rsa.PrivateKey) ([]byte, error) {
	//plaintext, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, priv, ciphertext, nil)
	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 32) //256bits
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func EncryptAES(msg, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
	ciphertext := gcm.Seal(nonce, nonce, msg, nil)

	return ciphertext, nil
}

func DecryptAES(ciphertext, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, err
    }

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	msg, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }

	return msg,nil
}

/*
func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("inexistent certificate file at " + path)
	}
	return x509.ParseCertificate(data)
}

func loadKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("inexistent key file at " + path)
	}
	privKey, err := x509.ParsePKCS1PrivateKey(data)
	if err != nil {
		return nil, err
	}
	return privKey.(*rsa.PrivateKey), nil
}
*/
func publicKeyToPem(publicKey *rsa.PublicKey) ([]byte, error) {
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

func privateKeyToPem(privateKey *rsa.PrivateKey) ([]byte, error) {
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	if pemKey == nil {
		return nil, errors.New("failed to encode key to PEM")
	}

	return pemKey, nil
}

func CertificateToPem(cert *x509.Certificate) ([]byte, error) {
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if pemCert == nil {
		return nil, errors.New("failed to encode certificate to PEM")
	}
	return pemCert, nil
}

func PemToCertificate(cert []byte ) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		return nil,errors.New("failed to parse certificate PEM")

	}
	return x509.ParseCertificate(block.Bytes)
}

func storeCertificate(cert *x509.Certificate, path string) ([]byte, error) {
	//serialize generated certificate into file
	pemCert, err := CertificateToPem(cert)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, pemCert, 0644); err != nil {
		return nil, err
	}

	return pemCert, nil
}

func storeKey(privateKey *rsa.PrivateKey, path string) ([]byte, error) {
	pemKey, err := privateKeyToPem(privateKey)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile("key.pem", pemKey, 0600); err != nil {
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
func generateCertificate(privateKey *rsa.PrivateKey, username string ,signingAuthority *x509.Certificate) (*x509.Certificate, error) {
	//each certificate needs a unique serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			//Organization: []string{"Partage"},
			Organization: []string{username},
		},
		//DNSNames:  []string{"localhost"}, //to make the certificate only valid for the localhost domain
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * 365 * 24 * time.Hour), //TODO: time validity of the certificate (currently valid for 1 year)

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

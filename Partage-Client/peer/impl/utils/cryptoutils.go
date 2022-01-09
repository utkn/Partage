package utils

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"strconv"

	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"math/big"
	mathRand "math/rand"
	"os"
	"strings"
	"time"
)

const dir = "Partage/Partage-Client/partage-storage/"
const cryptoDir =dir+"crypto/"
const certificatePath = cryptoDir + "cert.pem"
const keyPath = cryptoDir + "key.pem"
const signaturePath = cryptoDir + "publickey.signature"
const emailPath = dir+"my.email"
const CACertificatePath = cryptoDir + "CA/cert.pem"

func LoadCertificate(fromPersistentMem bool) (*tls.Certificate, error) {
	if fromPersistentMem {
		wd, _ := os.Getwd()
		rt := wd[:strings.Index(wd, "Partage")]

		cert, err := tls.LoadX509KeyPair(rt+certificatePath, rt+keyPath)
		if err != nil {
			//generate a new key-pair
			privateKey, _ := generateKey()

			keyPem, err := storePrivateKey(privateKey, rt+keyPath)
			if err != nil {
				return nil, err
			}

			//generate a new certificate from the newly-generated key-pair
			certificate, _ := GenerateCertificate(privateKey, nil) //SELF-SIGNED! TODO: ..to later be implemented with the CA

			certPem, err := storeCertificate(certificate, rt+certificatePath)
			if err != nil {
				return nil, err
			}

			cert, err = tls.X509KeyPair(certPem, keyPem)
			if err != nil {
				return nil, err
			}
			return &cert,nil
		}
		return &cert, nil

	} else {
		// Generate a new fresh self-signed certificate..(GOOD FOR TESTING PURPOSES!)
		// NOTE THAT THIS MODE DOESN'T STORE ANY key OR certificate
		privateKey, _ := generateKey()
		keyPem, err := PrivateKeyToPem(privateKey)
		if err != nil {
			return nil, err
		}

		certificate, _ := GenerateCertificate(privateKey, nil)
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

	return msg, nil
}

func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("inexistent certificate file at " + path)
	}

	return PemToCertificate(data)
}

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

func PrivateKeyToPem(privateKey *rsa.PrivateKey) ([]byte, error) {
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

func PemToCertificate(cert []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(cert)
	if block == nil {
		return nil, errors.New("failed to decode certificate PEM to ASN.1 DER data")

	}
	return x509.ParseCertificate(block.Bytes)
}

func StoreCertificate(cert *x509.Certificate) ([]byte, error) {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	return storeCertificate(cert, rt+certificatePath)
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

func storePrivateKey(privateKey *rsa.PrivateKey, path string) ([]byte, error) {
	pemKey, err := PrivateKeyToPem(privateKey)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, pemKey, 0600); err != nil {
		return nil, err
	}

	return pemKey, nil
}

func storePublicKey(k *rsa.PublicKey, path string) error {
	pemKey, err := publicKeyToPem(k)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, pemKey, 0600); err != nil {
		return err
	}
	return nil
}

//used to generate a private and public key using the P-256 elliptic curve
func generateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 1024)
}

func loadEmailFromFile() string{
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	data, err := os.ReadFile(rt+emailPath)
	if err != nil {
		return ""
	}
	return string(data)
}
//used to generate a signed certificate (if signingAuthority==nil, certificate is self-signed)
//returns new certificate as ASN.1 DER data (can be parsed to x509.Certificate object with x509.ParseCertificate(der []byte) function)
func GenerateCertificate(privateKey *rsa.PrivateKey, signingAuthority *x509.Certificate) (*x509.Certificate, error) {
	// Load e-mail from file!
	//email:=loadEmailFromFile() //TODO:!!!!!!!!
	mathRand.Seed(time.Now().Unix())
	email:="abdefg"+strconv.Itoa(mathRand.Intn(99999999))+"@gmail.com" //testing purposes
	//each certificate needs a unique serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * 365 * 24 * time.Hour), //(currently valid for 1 year)
		IsCA:                  true,
		BasicConstraintsValid: true,
		Subject: pkix.Name{
			Organization: []string{email},
		},
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

func IsSelfSigned(cert *x509.Certificate) bool {
	return cert.CheckSignatureFrom(cert) == nil
}

func TLSIsSelfSigned(cert *tls.Certificate) (bool, error) {
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		fmt.Println("[ERROR] parsing certificate from tls to x509")
		return false, err
	}
	return IsSelfSigned(x509Cert), nil
}

/*
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
} */

func loadPublicKeySignature(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func LoadPublicKeySignature() []byte {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	return loadPublicKeySignature(rt + signaturePath)
}

func LoadCACertificate() *x509.Certificate {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	cert, _ := loadCertificate(rt + CACertificatePath)
	return cert
}

func StoreCACertificate(cert *x509.Certificate) ([]byte, error) {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	return storeCertificate(cert, rt+CACertificatePath)
}

func StorePublicKeySignature(signature []byte) error {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	path := rt + signaturePath
	return os.WriteFile(path, signature, 0644)
}

func VerifyPublicKeySignature(publicKey *rsa.PublicKey, signature []byte, CAPublicKey *rsa.PublicKey) bool {
	pkBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		fmt.Println("[ERROR] marshaling public key to raw bytes")
		return false
	}
	hashed := Hash(pkBytes)
	return rsa.VerifyPKCS1v15(CAPublicKey, crypto.SHA256, hashed[:], signature) == nil
}

func HashPublicKey(pk *rsa.PublicKey) [32]byte {
	//stringify user's public key
	bytes, err := PublicKeyToBytes(pk)
	if err == nil {
		return Hash(bytes)
	}
	return [32]byte{}
}

func PublicKeyToBytes(publicKey *rsa.PublicKey) ([]byte, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, errors.New("failed to marshal public key")
	}
	return pubBytes, nil
}

func Hash(bytes []byte) [32]byte {
	return sha256.Sum256(bytes)
}

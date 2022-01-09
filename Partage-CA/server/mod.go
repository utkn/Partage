package server

//Persistent Storage
const StorageDir = "Partage-CA/storage/"
//CA's crypto details
const CryptoDir = StorageDir + "crypto/"
const CertificatePath = CryptoDir + "cert.pem"
const KeyPath = CryptoDir + "key.pem"
//Users public keys
const UsersPath = StorageDir + "users.db"
const EmailsPath = StorageDir + "emails.db"

//Server Address
const Addr = "127.0.0.1:1234"

type Server interface {
	Start() error
	Stop() error
	GetAddress() string
}

//SMS credentials

const SmtpUsername = "partage.register@gmail.com"
const SmtpPassword = "bilebtjkrcpstopa"
const SmtpHost = "smtp.gmail.com"
const SmtpPort = "587"

package content

import (
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
)

type PublicContent struct {
	AuthorID     string
	Text         string
	Timestamp    int64
	RefContentID string
}

type PrivateContent struct {
	// Contains the information about the content. If encrypted, Text field is hidden.
	PublicContent
	// Flag denoting whether the data is encrypted or not.
	Encrypted bool
	// List of data decryption keys asymmetrically encrypted with the public key of the recipients.
	DecryptionData []byte
	// List of the recipient user ids.
	RecipientList []string
	// Encrypted text.
	EncryptedData []byte
	// Signed everything.
	Signature []byte
}

func (textPost PublicContent) Encrypted(publicKeyMap map[[32]byte]*rsa.PublicKey) (PrivateContent, error) {
	// Generate Symmetric Encryption Key (AES-256)
	aesKey, err := utils.GenerateAESKey()
	if err != nil {
		return PrivateContent{}, err
	}
	// For each recipient, encrypt the aesKey with the user's RSA Public Key (associated with the user's TLS certificate)
	recipientMap := types.RecipientsMap{} //user_y:EncPK_x(aesKey),user_y:EncPK_y(aesKey),...
	var encryptedAESKey [128]byte
	var recipientList []string
	for hashedPK, pk := range publicKeyMap {
		if pk != nil {
			ciphertext, err := utils.EncryptWithPublicKey(aesKey, pk)
			if err != nil {
				return PrivateContent{}, err
			}
			copy(encryptedAESKey[:], ciphertext)
			if err != nil {
				return PrivateContent{}, err
			}
			recipientMap[hashedPK] = encryptedAESKey
			recipientList = append(recipientList, hex.EncodeToString(hashedPK[:]))
		}
	}
	// Encrypt Message with AES-256 key
	encryptedMsg, err := utils.EncryptAES([]byte(textPost.Text), aesKey)
	if err != nil {
		return PrivateContent{}, err
	}
	// Hide the contents.
	textPost.Text = "encrypted"
	decryptionData, err := recipientMap.Encode()
	if err != nil {
		return PrivateContent{}, err
	}
	//share Private Post
	return PrivateContent{
		Encrypted:      true,
		PublicContent:  textPost,
		RecipientList:  recipientList,
		DecryptionData: decryptionData,
		EncryptedData:  encryptedMsg,
	}, nil
}

func (textPost PublicContent) Unencrypted() PrivateContent {
	return PrivateContent{
		Encrypted:     false,
		PublicContent: textPost,
	}
}

func NewPublicContent(userID string, text string, timestamp int64, refContentID string) PublicContent {
	return PublicContent{
		AuthorID:     userID,
		Text:         text,
		Timestamp:    timestamp,
		RefContentID: refContentID,
	}
}

func ParseContent(bytes []byte) PrivateContent {
	var post PrivateContent
	_ = json.Unmarshal(bytes, &post)
	return post
}

func UnparseContent(value PrivateContent) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

func (p PrivateContent) decryptText(selfHashedPK [32]byte, selfPrivateKey *rsa.PrivateKey) (string, error) {
	// If not encrypted, automatically return the text post.
	if !p.Encrypted {
		return p.Text, nil
	}
	recipientsMap := types.RecipientsMap{}
	if err := recipientsMap.Decode(p.DecryptionData); err != nil {
		//fmt.Println(err)
		return "", err
	}
	// Process the embedded packet if we are in the recipient list.
	ciphertext, ok := recipientsMap[selfHashedPK]
	if !ok { //i'm not in the recipients list..
		return "", fmt.Errorf("not in the recipient list")
	}
	//decrypt the encrypted AES key, using my RSA private key
	aesKey, err := utils.DecryptWithPrivateKey(ciphertext[:], selfPrivateKey)
	if err != nil {
		return "", err
	}
	//decrypt bytes using the AES symmetric key
	msgBytes, err := utils.DecryptAES(p.EncryptedData, aesKey)
	if err != nil {
		return "", err
	}
	return string(msgBytes), nil
}

// Decrypted decrypts the private content and outputs a public displayable content.
func (p PrivateContent) Decrypted(selfHashedPK [32]byte, selfPrivateKey *rsa.PrivateKey) (PublicContent, error) {
	decryptedText, err := p.decryptText(selfHashedPK, selfPrivateKey)
	p.PublicContent.Text = decryptedText
	if err != nil {
		p.PublicContent.Text = err.Error()
	}
	return p.PublicContent, err
}

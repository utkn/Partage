package gossip

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) SendPost(content string) error{
	if len(content)>128{
		//limit max size of content
		return fmt.Errorf("post content exceeds size limit of 128 characters")
	}
	timestamp:=time.Now().UnixNano()
	//Hash(content||timestamp)
	hashed:=utils.Hash(append([]byte(content),[]byte(strconv.FormatInt(timestamp, 10))...))
	signature, err := rsa.SignPKCS1v15(rand.Reader, l.cryptography.GetPrivateKey(), crypto.SHA256, hashed[:])
	if err != nil {
		return err
	}
	post:=types.Post{
		Timestamp: timestamp,
		Content: content,
		Check: &transport.Validation{
			Signature: signature,
			SrcPublicKey: *l.cryptography.GetSignedPublicKey(),
		},
	}
	tMsg,err:=l.config.MessageRegistry.MarshalMessage(&post)
	if err!=nil{
		return err
	}
	
	return l.Broadcast(tMsg)
}

//recipients will be a slice containing the each recipient hashed public key
func (l *Layer) SendPrivatePost(msg transport.Message, recipients [][32]byte) error {
	// Generate Symmetric Encryption Key (AES-256)
	aesKey, err := utils.GenerateAESKey()
	if err != nil {
		return err
	}
	// For each recipient, encrypt the aesKey with the user's RSA Public Key (associated with the user's TLS certificate)
	users := types.RecipientsMap{} //user_y:EncPK_x(aesKey),user_y:EncPK_y(aesKey),...
	var encryptedAESKey [128]byte
	for _, username := range recipients {
		pk := l.cryptography.SearchPublicKey(username, l.cryptography.GetExpandingConf())
		if pk != nil {
			ciphertext, err := utils.EncryptWithPublicKey(aesKey, pk)
			if err != nil {
				return err
			}
			copy(encryptedAESKey[:], ciphertext)
			users[username] = encryptedAESKey
		}
	}

	// Encrypt Message with AES-256 key
	byteMsg, err := json.Marshal(msg) //from transport.Message to []byte
	if err != nil {
		return err
	}

	encryptedMsg, err := utils.EncryptAES(byteMsg, aesKey)
	if err != nil {
		return err
	}

	bytes, err := users.Encode()
	if err != nil {
		//fmt.Println(err)
		return err
	}
	//share Private Post
	privatePost := types.PrivatePost{
		Recipients: bytes,
		Msg:        encryptedMsg,
	}
	toSendMsg,err:=l.config.MessageRegistry.MarshalMessage(&privatePost)
	/*
	data, err := json.Marshal(&privatePost)
	if err != nil {
		return err
	}
	toSendMsg := transport.Message{
		Type:    privatePost.Name(),
		Payload: data,
	} */
	fmt.Println(l.GetAddress(), " Broadcasted privatePost to", len(users), "recipients!")

	return l.Broadcast(toSendMsg) //share
}

//--------HANDLERS
func (l *Layer) PostHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", l.GetAddress(), "is at PostHandler")
	post, ok := msg.(*types.Post)
	if !ok {
		return fmt.Errorf("could not parse the private post message")
	}
	//check validity of post
	if len(post.Content)>128{
		return fmt.Errorf("post content exceeds size limit of 128 characters")
	}

	//1- Check if the Public Key of the user who signed the message is valid (signed by trusted CA)
	srcPKBytes, err := x509.MarshalPKIXPublicKey(post.Check.SrcPublicKey.PublicKey)
	if err != nil {
		return err
	}
	hashedPK := utils.Hash(srcPKBytes)
	if err:=rsa.VerifyPKCS1v15(l.cryptography.GetCAPublicKey(),crypto.SHA256,hashedPK[:],post.Check.SrcPublicKey.Signature); err!=nil{
		//invalid SignedPublicKey (not signed by trusted CA)
		return fmt.Errorf("post's public key is not signed by trusted CA")
	}

	//2- Check if Hash(packet.Msg||packet.Header.Source) was signed by src's private key
	hashed := utils.Hash(append([]byte(post.Content),[]byte(strconv.FormatInt(post.Timestamp, 10))...))
	if rsa.VerifyPKCS1v15(post.Check.SrcPublicKey.PublicKey,crypto.SHA256,hashed[:],post.Check.Signature)!=nil{
		//invalid signature
		return fmt.Errorf("message has invalid signature")
	}

	//VALID!
	fmt.Println("Received: ",post)
	return nil
}

//PrivatePost Handler (Note: didnt put it on handles.go file to avoid mixing it with the gossip handlers)
func (l *Layer) PrivatePostHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("handler", l.GetAddress(), "is at PrivatePostHandler")
	privateMsg, ok := msg.(*types.PrivatePost)
	if !ok {
		return fmt.Errorf("could not parse the private post message")
	}
	recipientsMap:= types.RecipientsMap{}
	if err:=recipientsMap.Decode(privateMsg.Recipients); err!=nil{
		//fmt.Println(err)
		return err
	}
	// Process the embedded packet if we are in the recipient list.
	ciphertext, ok := recipientsMap[l.cryptography.GetHashedPublicKey()]
	if !ok { //i'm not in the recipients list..
		return nil
	}
	fmt.Println(l.GetAddress(), " rcvd a private post for me!")
	//decrypt the encrypted AES key, using my RSA private key
	aesKey, err := utils.DecryptWithPrivateKey(ciphertext[:], l.cryptography.GetPrivateKey())
	if err != nil {
		return err
	}

	//decrypt bytes using the AES symmetric key
	msgBytes, err := utils.DecryptAES(privateMsg.Msg, aesKey)
	if err != nil {
		return err
	}
	//convert from bytes to transport.Message type
	var transportMsg transport.Message

	if err := json.Unmarshal(msgBytes, &transportMsg); err != nil {
		return err
	}
	fmt.Println(l.GetAddress(), "rcvd the following private post:", transportMsg)
	//transport.Message
	transpPacket := transport.Packet{
		Header: pkt.Header,
		Msg:    &transportMsg, //privateMsg.Msg,
	}

	l.config.MessageRegistry.ProcessPacket(transpPacket.Copy())

	return nil
}
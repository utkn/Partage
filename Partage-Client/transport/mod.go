package transport

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rs/xid"
)

// Factory defines the general function to create a network.
type Factory func() Transport

// Transport defines the primitives to handler a layer 4 transport.
type Transport interface {
	CreateSocket(address string) (ClosableSocket, error)
}

// Socket describes the primitives for a socket communication element
//
// - Implemented in HW0
type Socket interface {
	// Send sends a msg to the destination. If the timeout is reached without
	// message received, returns a TimeoutErr. A value of 0 means no timeout.
	Send(dest string, pkt Packet, timeout time.Duration) error

	// Recv blocks until a packet is received, or the timeout is reached. In the
	// case the timeout is reached, returns a TimeoutErr. A value of 0 means no
	// timeout.
	Recv(timeout time.Duration) (Packet, error)

	// GetAddress returns the address assigned. Can be useful in the case one
	// provided a :0 address, which makes the system use a random free port.
	GetAddress() string

	// GetIns must return all the messages received so far.
	GetIns() []Packet

	// GetOuts must return all the messages sent so far.
	GetOuts() []Packet
}

// ClosableSocket augments the Socket interface with a close function. We
// differentiate it because a gossiper shouldn't have access to the close
// function of a socket.
type ClosableSocket interface {
	Socket

	// Close closes the connection. It returns an error if the socket is already
	// closed.
	Close() error
}

// TimeoutErr is a type of error used by the network interface if a timeout is
// reached when receiving a packet.
type TimeoutErr time.Duration

// Error implements error. Returns the error string.
func (err TimeoutErr) Error() string {
	return fmt.Sprintf("timeout reached after %d", err)
}

// Is implements error.
func (TimeoutErr) Is(err error) bool {
	_, ok := err.(TimeoutErr)
	return ok
}

// NewHeader returns a new header with initialized fields. If the ttl value is 0
// then it uses the maximum possible ttl value.
func NewHeader(source, relay, destination string, ttl uint) Header {
	if ttl == 0 {
		ttl = math.MaxUint64
	}

	return Header{
		PacketID:    xid.New().String(),
		TTL:         ttl,
		Timestamp:   time.Now().UnixNano(),
		Source:      source,
		RelayedBy:   relay,
		Destination: destination,
	}
}

// Packet is a type of message sent over the network
type Packet struct {
	Header *Header

	Msg *Message
}

// Marshal transforms a packet to something that can be sent over the network.
func (p Packet) Marshal() ([]byte, error) {
	return json.Marshal(&p)
}

// Unmarshal transforms a marshaled packet to an actual packet. Example
// creating a new packet out of a buffer:
//   var packet Packet
//   packet.Unmarshal(buf)
func (p *Packet) Unmarshal(buf []byte) error {
	return json.Unmarshal(buf, p)
}

// Copy returns a copy of the packet
func (p Packet) Copy() Packet {
	h := p.Header.Copy()
	m := p.Msg.Copy()

	return Packet{
		Header: &h,
		Msg:    &m,
	}
}

// Header contains the metadata of a packet needed for its transport.
type Header struct {
	// PacketID is a unique packet identifier. Used for debug purposes.
	PacketID string

	// TTL is the Time To Live of a packet, which is set when the packet is
	// created and decremented by each relayer. A packet is dropped when its TTL
	// is equal to 0.
	TTL uint

	// Timestamp is the creation timetamp of the packet, in nanosecond.
	Timestamp int64

	// Source is the address of the packet's creator.
	Source string

	// RelayedBy is the address of the node that sends the packet. It can be the
	// originator of the packet, in which case Source==RelayedBy, or the address
	// of a node relaying the packet. Each node should update this field when it
	// relays a packet.
	//
	// - Implemented in HW1
	RelayedBy string

	// Destination is empty in the case of a broadcast, otherwise contains the
	// destination address.
	Destination string

	//---PARTAGE allows to check integrity and authenticity check
	Check *Validation
}

func (h Header) String() string {
	out := new(strings.Builder)

	timeStr := time.Unix(0, h.Timestamp).Format("04:04:05.000000000")

	fmt.Fprintf(out, "ID: %s\n", h.PacketID)
	fmt.Fprintf(out, "Timestamp: %s\n", timeStr)
	fmt.Fprintf(out, "Source: %s\n", h.Source)
	fmt.Fprintf(out, "RelayedBy: %s\n", h.RelayedBy)
	fmt.Fprintf(out, "Destination: %s\n", h.Destination)
	fmt.Fprintf(out, "TTL: %d\n", h.TTL)

	return out.String()
}

// Copy returns the copy of header
func (h Header) Copy() Header {
	return h
}

// HTML returns an HTML representation of a header.
func (h Header) HTML() string {
	return strings.ReplaceAll(h.String(), "\n", "<br/>")
}

// Message defines the type of message sent over the network. Payload should be
// a json marshalled representation of a types.Message, and Type the
// corresponding message name, available with types.Message.Name().
type Message struct {
	Type    string
	Payload json.RawMessage
}

// Copy returns a copy of the message
func (m Message) Copy() Message {
	return Message{
		Type:    m.Type,
		Payload: append([]byte{}, m.Payload...),
	}
}

// ByPacketID define a type to sort packets by ByPacketID
type ByPacketID []Packet

func (p ByPacketID) Len() int {
	return len(p)
}

func (p ByPacketID) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p ByPacketID) Less(i, j int) bool {
	return p[i].Header.PacketID < p[j].Header.PacketID
}

//================PARTAGE
//---------
type SignedPublicKey struct {
	PublicKey *rsa.PublicKey
	Signature []byte
}

type Validation struct {
	Signature    []byte
	SrcPublicKey SignedPublicKey
}

func (p *Packet) AddValidation(myPrivateKey *rsa.PrivateKey, mySignedPublicKey *SignedPublicKey) error {
	//Adds a validation check to the Packet's header (provides integrity and authenticity check!)
	byteMsg, err := json.Marshal(p.Msg)
	if err != nil {
		return err
	}
	// Hash(packet.Msg||packet.Header.Source)
	hashedContent := Hash(append(byteMsg, []byte(p.Header.Source)...))
	signature, err := rsa.SignPKCS1v15(rand.Reader, myPrivateKey, crypto.SHA256, hashedContent[:])
	if err != nil {
		return err
	}

	p.Header.Check = &Validation{
		Signature:    signature,
		SrcPublicKey: *mySignedPublicKey,
	}

	return nil
}

func (p *Packet) Validate(publicKeyCA *rsa.PublicKey) error {
	if p.Header.Check == nil {
		if p.Msg.Type == "searchpkrequest" || p.Msg.Type == "searchpkreply" || p.Msg.Type == "datarequest" || p.Msg.Type == "datareply" || p.Msg.Type == "searchrequest" || p.Msg.Type == "searchreply" {
			return nil
			//exceptions!
			//no need to check for integrity or authenticity in searchpkrequest or searchpkreply
			//since the content of the msg is what's really important..neither of these types implements Validation
		}
		return fmt.Errorf("pkt.Header has empty validation check")
	}

	//1- Check if the Public Key of the user who signed the message is valid (signed by trusted CA)
	srcPKBytes, err := x509.MarshalPKIXPublicKey(p.Header.Check.SrcPublicKey.PublicKey)
	if err != nil {
		return err
	}
	hashedSrcPK := Hash(srcPKBytes)
	if err := rsa.VerifyPKCS1v15(publicKeyCA, crypto.SHA256, hashedSrcPK[:], p.Header.Check.SrcPublicKey.Signature); err != nil {
		//invalid SignedPublicKey (not signed by trusted CA)
		return fmt.Errorf("src public key is not signed by trusted CA")
	}

	//2- Check if Hash(packet.Msg||packet.Header.Source) was signed by src's private key
	byteMsg, _ := json.Marshal(p.Msg)
	hashedContent := Hash(append(byteMsg, []byte(p.Header.Source)...))
	if rsa.VerifyPKCS1v15(p.Header.Check.SrcPublicKey.PublicKey, crypto.SHA256, hashedContent[:], p.Header.Check.Signature) != nil {
		//invalid signature
		return fmt.Errorf("message is not signed by src private key")
	}

	//VALID!
	return nil
}

func (k *SignedPublicKey) Encode() ([]byte, error) {
	return json.Marshal(k)
}

func (k *SignedPublicKey) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &k)
}

func Hash(bytes []byte) [32]byte {
	return sha256.Sum256(bytes)
}

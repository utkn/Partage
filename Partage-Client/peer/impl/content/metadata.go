package content

import (
	"crypto"
	"encoding/json"
	"fmt"
)

// Metadata represents the information about any content on a feed. The information to handle a content are
// stored in the Data field.
type Metadata struct {
	FeedUserID string
	Type       Type
	ContentID  string
	// Used by Comments, Reactions...
	RefContentID string
	Timestamp    int64
	Data         []byte
	Signature    []byte
}

func ParseMetadata(metadataBytes []byte) Metadata {
	var data Metadata
	_ = json.Unmarshal(metadataBytes, &data)
	return data
}

func UnparseMetadata(value Metadata) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

func HashMetadata(index uint, uniqID string, metadata Metadata, prevHash []byte) []byte {
	h := crypto.SHA256.New()
	metadataBytes := UnparseMetadata(metadata)
	h.Write([]byte(fmt.Sprint(index)))
	h.Write([]byte(uniqID))
	h.Write(metadataBytes)
	h.Write(prevHash)
	hashSlice := h.Sum(nil)
	return hashSlice
}

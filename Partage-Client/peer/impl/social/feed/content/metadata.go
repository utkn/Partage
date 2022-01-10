package content

import (
	"encoding/json"
)

// Metadata represents the information about any content on a feed. The information to handle a content are
// stored in the Data field.
type Metadata struct {
	FeedUserID string
	Type       ContentType
	ContentID  string
	Timestamp  int
	Data       []byte
	Signature  []byte
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

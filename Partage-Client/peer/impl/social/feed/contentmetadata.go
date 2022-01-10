package feed

import "encoding/json"

// ContentMetadata represents the information about a post on a feed. The post contents are stored as data blocks addressed
// by their content id.
type ContentMetadata struct {
	FeedUserID string
	Type       string
	ContentID  string
	Timestamp  int
	Signature  []byte
}

func ParseCustomPaxosValue(customValueBytes []byte) ContentMetadata {
	var data ContentMetadata
	_ = json.Unmarshal(customValueBytes, &data)
	return data
}

func MakeCustomPaxosValue(value ContentMetadata) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

package feed

import "encoding/json"

// ContentMetadata represents the information about a post on a feed. The contents are stored as data blocks addressed
// by their content id.
type ContentMetadata struct {
	FeedUserID string
	Type       ContentType
	ContentID  string
	Timestamp  int
	Data       []byte
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

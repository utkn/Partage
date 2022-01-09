package feed

import "encoding/json"

// PostInfo represents the information about a post on a feed. The post contents are stored as data blocks addressed
// by their content id.
type PostInfo struct {
	FeedUserID    string
	PostType      string
	PostContentID string
	Signature     []byte
}

func ParseCustomPaxosValue(customValueBytes []byte) PostInfo {
	var data PostInfo
	_ = json.Unmarshal(customValueBytes, &data)
	return data
}

func MakeCustomPaxosValue(value PostInfo) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

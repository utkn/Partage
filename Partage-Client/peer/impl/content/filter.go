package content

import (
	"encoding/json"
	"go.dedis.ch/cs438/storage"
)

// Filter represents a filter that can be used to search content through the network.
type Filter struct {
	// MaxTime denotes the highest end of the time range. Setting to 0 disables it.
	MaxTime int64
	// MinTime denotes the lowest end of the time range. Setting to 0 disables it.
	MinTime int64
	// OwnerIDs filters by the owner user ids. Setting to empty list (nil) disables it.
	OwnerIDs []string
	// Types filters by the list of types. Setting to empty list (nil) disables it.
	Types []Type
	// RefContentID filters by the reference id. Used for comments, reactions etc. Setting to "" disables it.
	RefContentID string
}

func ParseContentFilter(contentFilterBytes []byte) Filter {
	var data Filter
	_ = json.Unmarshal(contentFilterBytes, &data)
	return data

}

func UnparseContentFilter(value Filter) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

// Match returns true if the given metadata matches the filter.
func (c Filter) Match(metadata Metadata) bool {
	// Check against filtered min time.
	if c.MinTime > 0 && metadata.Timestamp < c.MinTime {
		return false
	}
	// Check against filtered max time.
	if c.MaxTime > 0 && metadata.Timestamp > c.MaxTime {
		return false
	}
	// Check against referenced id.
	if c.RefContentID != "" && c.RefContentID != metadata.RefContentID {
		return false
	}
	// Check against allowed owners.
	ownerMatch := false
	if len(c.OwnerIDs) == 0 {
		ownerMatch = true
	}
	for _, acceptableOwnerID := range c.OwnerIDs {
		if metadata.FeedUserID == acceptableOwnerID {
			ownerMatch = true
			break
		}
	}
	if !ownerMatch {
		return false
	}
	// Check against allowed types.
	typeMatch := false
	if len(c.Types) == 0 {
		typeMatch = true
	}
	for _, acceptableType := range c.Types {
		if metadata.Type == acceptableType {
			typeMatch = true
			break
		}
	}
	if !typeMatch {
		return false
	}
	return true
}

// GetMatchedContentMetadatas searches through a meta data store (content id -> content.MetaData) and returns the metadatas
// that match.
func GetMatchedContentMetadatas(metadataStore storage.Store, filter Filter) []Metadata {
	var allMatches []Metadata
	metadataStore.ForEach(
		func(contentID string, metadataBytes []byte) bool {
			metadata := ParseMetadata(metadataBytes)
			match := filter.Match(metadata)
			if match {
				allMatches = append(allMatches, metadata)
			}
			return true
		})
	return allMatches
}

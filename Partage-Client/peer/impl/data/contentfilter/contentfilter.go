package contentfilter

import (
	"encoding/json"
	"go.dedis.ch/cs438/peer/impl/social/feed/content"
	"go.dedis.ch/cs438/storage"
)

// ContentFilter represents a filter that can be used to search content through the network.
type ContentFilter struct {
	MaxTime  int
	MinTime  int
	OwnerIDs []string
	Types    []content.ContentType
}

func ParseContentFilter(contentFilterBytes []byte) ContentFilter {
	var data ContentFilter
	_ = json.Unmarshal(contentFilterBytes, &data)
	return data

}

func UnparseContentFilter(value ContentFilter) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

// Match returns true if the given metadata matches the filter.
func (c ContentFilter) Match(metadata content.Metadata) bool {
	// Check against filtered min time.
	if c.MinTime > 0 && metadata.Timestamp < c.MinTime {
		return false
	}
	// Check against filtered max time.
	if c.MaxTime > 0 && metadata.Timestamp > c.MaxTime {
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
func GetMatchedContentMetadatas(metadataStore storage.Store, filter ContentFilter) []content.Metadata {
	var allMatches []content.Metadata
	metadataStore.ForEach(
		func(contentID string, metadataBytes []byte) bool {
			metadata := content.ParseMetadata(metadataBytes)
			match := filter.Match(metadata)
			if match {
				allMatches = append(allMatches, metadata)
			}
			return true
		})
	return allMatches
}

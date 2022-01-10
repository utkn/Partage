package data

import (
	"fmt"
	"github.com/rs/xid"
	content2 "go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"time"
)

// SearchAllPostContent returns the all the matched content ids.
func (l *Layer) SearchAllPostContent(filter content2.Filter, budget uint, timeout time.Duration) ([]string, error) {
	utils.PrintDebug("data", l.GetAddress(), "is initiating a search...")
	localMatches := content2.GetMatchedContentMetadatas(l.config.BlockchainStorage.GetStore("metadata"), filter)
	allMatchesSet := make(map[string]struct{})
	for _, m := range localMatches {
		allMatchesSet[m.ContentID] = struct{}{}
	}
	neighbors := l.network.GetNeighbors()
	budgetMap := utils.DistributeBudget(budget, neighbors)
	// Create a search request id.
	searchRequestID := xid.New().String()
	// Save the search request id.
	l.catalogLock.Lock()
	l.processedSearchRequests[searchRequestID] = struct{}{}
	l.catalogLock.Unlock()
	// For each neighbor with a non-zero budget, send a search request.
	for neighbor, budget := range budgetMap {
		// Create the search request.
		msg := SearchContentRequestMessage{
			RequestID:     searchRequestID,
			Origin:        l.GetAddress(),
			ContentFilter: content2.UnparseContentFilter(filter),
			Budget:        budget,
		}
		transpMsg, _ := l.config.MessageRegistry.MarshalMessage(msg)
		// Send the search request.
		err := l.cryptography.Unicast(neighbor, transpMsg)
		if err != nil {
			utils.PrintDebug("data", "Could not unicast the search request")
			continue
		}
	}
	// Collect the received responses.
	responses := l.notification.MultiResponseCollector(searchRequestID, timeout, -1)
	// Construct the set of unique matches.
	for _, resp := range responses {
		searchResp := resp.(*SearchContentReplyMessage)
		for _, fileInfo := range searchResp.Responses {
			allMatchesSet[fileInfo.ContentID] = struct{}{}
		}
	}
	// Convert the set into a list.
	allMatches := make([]string, 0, len(allMatchesSet))
	for m := range allMatchesSet {
		allMatches = append(allMatches, m)
	}
	return allMatches, nil
}

func (l *Layer) DownloadContent(contentID string) ([]byte, error) {
	// First, get the metadata with the given content id.
	metadataBytes := l.config.BlockchainStorage.GetStore("metadata").Get(contentID)
	if metadataBytes == nil {
		return nil, fmt.Errorf("unknown content id")
	}
	metadata := content2.ParseMetadata(metadataBytes)
	// Then, get the metahash associated with the given post content.
	metahash, _ := content2.ParseTextPostMetadata(metadata)
	return l.Download(metahash)
}

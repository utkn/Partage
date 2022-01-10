package data

import (
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/data/contentfilter"
	"go.dedis.ch/cs438/peer/impl/social/feed/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) SearchPostContentReplyMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at SearchPostContentReplyMessageHandler")
	searchReplyMsg, ok := msg.(*SearchContentReplyMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search content reply msg")
	}
	for _, resp := range searchReplyMsg.Responses {
		l.UpdateCatalog(resp.Metahash, pkt.Header.Source)
		for _, chunkHash := range resp.Chunks {
			if chunkHash == nil {
				continue
			}
			l.UpdateCatalog(string(chunkHash), pkt.Header.Source)
		}
	}
	l.notification.DispatchResponse(searchReplyMsg.RequestID, msg)
	return nil
}

func (l *Layer) SearchPostContentRequestMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at SearchPostContentRequestMessageHandler")
	searchRequestMsg, ok := msg.(*SearchContentRequestMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search request msg")
	}
	l.catalogLock.Lock()
	_, alreadyProcessed := l.processedSearchRequests[searchRequestMsg.RequestID]
	// Duplicate request received.
	if alreadyProcessed {
		utils.PrintDebug("data", l.GetAddress(), "is ignoring the duplicate request.")
		l.catalogLock.Unlock()
		return nil
	}
	// Save the request id.
	l.processedSearchRequests[searchRequestMsg.RequestID] = struct{}{}
	l.catalogLock.Unlock()
	neighbors := l.network.GetNeighbors()
	delete(neighbors, pkt.Header.RelayedBy)
	budgetMap := utils.DistributeBudget(searchRequestMsg.Budget-1, neighbors)
	// Forward with the correct budget.
	for neighbor, budget := range budgetMap {
		searchRequestMsg.Budget = budget
		transpMsg, _ := l.config.MessageRegistry.MarshalMessage(searchRequestMsg)
		_ = l.cryptography.Route(searchRequestMsg.Origin, neighbor, neighbor, transpMsg)
	}
	var fileInfos []ContentInfo
	contentFilter := contentfilter.ParseContentFilter(searchRequestMsg.ContentFilter)
	matchedMetadatas := contentfilter.GetMatchedContentMetadatas(l.config.BlockchainStorage.GetStore("metadata"), contentFilter)
	for _, metadata := range matchedMetadatas {
		// Extract the metahash from the post content metadata.
		metahash, _ := content.ParseTextPostMetadata(metadata)
		_, chunkHashes, err := utils.GetLocalChunks(l.config.Storage.GetDataBlobStore(), metahash, peer.MetafileSep)
		// Skip if we couldn't get the chunks, which means metafile is not in the store.
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, ContentInfo{
			ContentID: metadata.ContentID,
			Metahash:  metahash,
			Chunks:    chunkHashes,
		})
	}
	searchReplyMsg := SearchContentReplyMessage{
		RequestID: searchRequestMsg.RequestID,
		Responses: fileInfos,
	}
	transpMsg, _ := l.config.MessageRegistry.MarshalMessage(searchReplyMsg)
	utils.PrintDebug("data", l.GetAddress(), "is sending back a search reply to", pkt.Header.Source, "with", fileInfos)
	_ = l.cryptography.Route(l.GetAddress(), pkt.Header.RelayedBy, searchRequestMsg.Origin, transpMsg)
	return nil
}

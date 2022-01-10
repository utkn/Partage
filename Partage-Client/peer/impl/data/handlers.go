package data

import (
	"fmt"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (l *Layer) RegisterHandlers() {
	l.config.MessageRegistry.RegisterMessageCallback(SearchContentReplyMessage{}, l.SearchPostContentReplyMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(SearchContentRequestMessage{}, l.SearchPostContentRequestMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.DataReplyMessage{}, l.DataReplyMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.DataRequestMessage{}, l.DataRequestMessageHandler)
	// The following handlers are left for backwards compatibility.
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchReplyMessage{}, l.SearchReplyMessageHandler)
	l.config.MessageRegistry.RegisterMessageCallback(types.SearchRequestMessage{}, l.SearchRequestMessageHandler)
}

func (l *Layer) DataReplyMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at DataReplyMessageHandler")
	dataReplyMsg, ok := msg.(*types.DataReplyMessage)
	if !ok {
		return fmt.Errorf("could not parse the received data reply message")
	}
	l.notification.DispatchResponse(dataReplyMsg.RequestID, msg)
	return nil
}

func (l *Layer) DataRequestMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at DataRequestMessageHandler")
	dataRequestMsg, ok := msg.(*types.DataRequestMessage)
	if !ok {
		return fmt.Errorf("could not parse the received data request message")
	}
	chunk := l.config.Storage.GetDataBlobStore().Get(dataRequestMsg.Key)
	dataReplyMsg := types.DataReplyMessage{
		RequestID: dataRequestMsg.RequestID,
		Key:       dataRequestMsg.Key,
		Value:     chunk,
	}
	dataReplyMsgTransp, err := l.config.MessageRegistry.MarshalMessage(dataReplyMsg)
	if err != nil {
		return fmt.Errorf("could not parse the responding data reply message")
	}
	err = l.network.Unicast(pkt.Header.Source, dataReplyMsgTransp)
	return err
}

func (l *Layer) SearchReplyMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at SearchReplyMessageHandler")
	searchReplyMsg, ok := msg.(*types.SearchReplyMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search reply msg")
	}
	for _, resp := range searchReplyMsg.Responses {
		l.config.Storage.GetNamingStore().Set(resp.Name, []byte(resp.Metahash))
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

func (l *Layer) SearchRequestMessageHandler(msg types.Message, pkt transport.Packet) error {
	utils.PrintDebug("data", l.GetAddress(), "is at SearchRequestMessageHandler")
	searchRequestMsg, ok := msg.(*types.SearchRequestMessage)
	if !ok {
		return fmt.Errorf("could not parse the received search request msg")
	}
	l.catalogLock.Lock()
	_, ok = l.processedSearchRequests[searchRequestMsg.RequestID]
	// Duplicate request received.
	if ok {
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
		_ = l.network.Route(searchRequestMsg.Origin, neighbor, neighbor, transpMsg)
	}
	var fileInfos []types.FileInfo
	matchedNames := utils.GetMatchedNames(l.config.Storage.GetNamingStore(), searchRequestMsg.Pattern)
	for _, matchedName := range matchedNames {
		metahash := string(l.config.Storage.GetNamingStore().Get(matchedName))
		_, chunkHashes, err := utils.GetLocalChunks(l.config.Storage.GetDataBlobStore(), metahash, peer.MetafileSep)
		// Skip if we couldn't get the chunks, which means metafile is not in the store.
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, types.FileInfo{
			Name:     matchedName,
			Metahash: metahash,
			Chunks:   chunkHashes,
		})
	}
	searchReplyMsg := types.SearchReplyMessage{
		RequestID: searchRequestMsg.RequestID,
		Responses: fileInfos,
	}
	transpMsg, _ := l.config.MessageRegistry.MarshalMessage(searchReplyMsg)
	utils.PrintDebug("data", l.GetAddress(), "is sending back a search reply to", pkt.Header.Source, "with", fileInfos)
	_ = l.network.Route(l.GetAddress(), pkt.Header.RelayedBy, searchRequestMsg.Origin, transpMsg)
	return nil
}

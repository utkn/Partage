package data

import (
	"fmt"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/consensus"
	"go.dedis.ch/cs438/peer/impl/cryptography"
	"go.dedis.ch/cs438/peer/impl/gossip"
	"go.dedis.ch/cs438/peer/impl/network"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Layer struct {
	gossip       *gossip.Layer
	consensus    *consensus.Layer
	network      *network.Layer
	cryptography *cryptography.Layer

	notification            *utils.AsyncNotificationHandler
	config                  *peer.Configuration
	catalog                 peer.Catalog
	catalogLock             sync.Mutex
	processedSearchRequests map[string]struct{}
}

func Construct(gossip *gossip.Layer, consensus *consensus.Layer, network *network.Layer,
	crypto *cryptography.Layer,
	config *peer.Configuration) *Layer {
	return &Layer{
		gossip:                  gossip,
		consensus:               consensus,
		network:                 network,
		cryptography:            crypto,
		notification:            utils.NewAsyncNotificationHandler(),
		config:                  config,
		catalog:                 make(peer.Catalog),
		processedSearchRequests: make(map[string]struct{}),
	}
}

func (l *Layer) GetAddress() string {
	return l.network.GetAddress()
}

func (l *Layer) RemoteDataRequest(dest string, msg types.DataRequestMessage) ([]byte, error) {
	transpMsg, err := l.config.MessageRegistry.MarshalMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("could not marshal data request message: %w", err)
	}
	replyTimeout := l.config.BackoffDataRequest.Initial
	for i := uint(0); i < l.config.BackoffDataRequest.Retry; i++ {
		// Unicast the data request message to the given destination peer -- using the routing table.
		err = l.network.Unicast(dest, transpMsg)
		if err != nil {
			return nil, fmt.Errorf("could not unicast the data request: %w", err)
		}
		// Block until we receive the response.
		reply := l.notification.ResponseCollector(msg.RequestID, replyTimeout)
		if reply != nil && reply.(*types.DataReplyMessage).Value == nil {
			return nil, fmt.Errorf("peer replied with an empty chunk")
		}
		if reply != nil && reply.(*types.DataReplyMessage).Value != nil {
			return reply.(*types.DataReplyMessage).Value, nil
		}
		// Increase the reply timeout by the factor.
		replyTimeout = replyTimeout * time.Duration(l.config.BackoffDataRequest.Factor)
	}
	return nil, fmt.Errorf("could not get a reply")
}

func (l *Layer) Upload(data io.Reader) (metahash string, err error) {
	chunks, metahash, err := utils.Chunkify(l.config.ChunkSize, peer.MetafileSep, data)
	if err != nil {
		return "", err
	}
	for h, c := range chunks {
		l.config.Storage.GetDataBlobStore().Set(h, c)
	}
	return metahash, err
}

func (l *Layer) Download(metahash string) ([]byte, error) {
	metafileBytes, err := l.AcquireData(metahash)
	if err != nil {
		return nil, fmt.Errorf("could not acquire the metafile: %w", err)
	}
	var recoveredData []byte
	chunkHashes := strings.Split(string(metafileBytes), peer.MetafileSep)
	for _, chunkHash := range chunkHashes {
		chunk, err := l.AcquireData(chunkHash)
		if err != nil {
			return nil, fmt.Errorf("could not acquire a chunk: %w", err)
		}
		recoveredData = append(recoveredData, chunk...)
	}
	return recoveredData, nil
}

func (l *Layer) AcquireData(hash string) ([]byte, error) {
	// First, try to find the data locally.
	chunk := l.config.Storage.GetDataBlobStore().Get(hash)
	if chunk != nil {
		return chunk, nil
	}
	// If the data does not exist locally, we will get it from a remote peer. Find the owners
	// of the data in our catalog.
	ownerPeers, ok := l.catalog[hash]
	if !ok || len(ownerPeers) == 0 {
		return nil, fmt.Errorf("no way to access the chunk")
	}
	// Choose a random peer from the catalog.
	randomPeer, err := utils.ChooseRandom(ownerPeers, nil)
	if err != nil {
		return nil, fmt.Errorf("no way to access the chunk: %w", err)
	}
	msg := types.DataRequestMessage{
		RequestID: xid.New().String(),
		Key:       hash,
	}
	// Get the data remotely.
	remoteData, err := l.RemoteDataRequest(randomPeer, msg)
	if err != nil {
		return nil, err
	}
	// Save the remote data locally.
	l.config.Storage.GetDataBlobStore().Set(hash, remoteData)
	return remoteData, nil
}

// Tag implements peer.DataSharing
func (l *Layer) Tag(name string, mh string) error {
	if l.config.Storage.GetNamingStore().Get(name) != nil {
		return fmt.Errorf("name taken")
	}
	// If there are <= 1 many peers, no need to invoke the consensus protocol.
	if l.config.TotalPeers <= 1 {
		l.config.Storage.GetNamingStore().Set(name, []byte(mh))
		return nil
	}
	_, err := l.consensus.Propose(types.PaxosValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	})
	if err != nil {
		return err
	}
	return nil
}

// Resolve implements peer.DataSharing
func (l *Layer) Resolve(name string) string {
	metahash := l.config.Storage.GetNamingStore().Get(name)
	if metahash == nil {
		return ""
	}
	return string(metahash)
}

// GetCatalog implements peer.DataSharing
func (l *Layer) GetCatalog() peer.Catalog {
	l.catalogLock.Lock()
	defer l.catalogLock.Unlock()
	copied := make(map[string]map[string]struct{})
	for k, v := range l.catalog {
		copied[k] = make(map[string]struct{})
		for p := range v {
			copied[k][p] = struct{}{}
		}
	}
	return copied
}

// UpdateCatalog implements peer.DataSharing
func (l *Layer) UpdateCatalog(key string, peer string) {
	l.catalogLock.Lock()
	defer l.catalogLock.Unlock()
	_, ok := l.catalog[key]
	if !ok {
		l.catalog[key] = make(map[string]struct{})
	}
	l.catalog[key][peer] = struct{}{}
}

// SearchAll implements peer.DataSharing
func (l *Layer) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {
	localMatches := utils.GetMatchedNames(l.config.Storage.GetNamingStore(), reg.String())
	allMatchesSet := make(map[string]struct{})
	for _, m := range localMatches {
		allMatchesSet[m] = struct{}{}
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
		msg := types.SearchRequestMessage{
			RequestID: searchRequestID,
			Origin:    l.GetAddress(),
			Pattern:   reg.String(),
			Budget:    budget,
		}
		transpMsg, _ := l.config.MessageRegistry.MarshalMessage(msg)
		// Send the search request.
		err = l.network.Unicast(neighbor, transpMsg)
		if err != nil {
			utils.PrintDebug("data", "Could not unicast the search request")
			continue
		}
	}
	// Collect the received responses.
	responses := l.notification.MultiResponseCollector(searchRequestID, timeout, -1)
	// Construct the set of unique matches.
	for _, resp := range responses {
		searchResp := resp.(*types.SearchReplyMessage)
		for _, fileInfo := range searchResp.Responses {
			allMatchesSet[fileInfo.Name] = struct{}{}
		}
	}
	// Convert the set into a list.
	allMatches := make([]string, 0, len(allMatchesSet))
	for m := range allMatchesSet {
		allMatches = append(allMatches, m)
	}
	return allMatches, nil
}

// SearchFirst implements peer.DataSharing
func (l *Layer) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (string, error) {
	// First look for a full match locally.
	matchedNames := utils.GetMatchedNames(l.config.Storage.GetNamingStore(), pattern.String())
	for _, matchedName := range matchedNames {
		metahash := string(l.config.Storage.GetNamingStore().Get(matchedName))
		if utils.IsFullMatchLocally(l.config.Storage.GetDataBlobStore(), metahash, peer.MetafileSep) {
			return matchedName, nil
		}
	}
	// Initiate the expanding ring search.
	budget := conf.Initial
	for i := uint(0); i < conf.Retry; i++ {
		// Create the search request id.
		searchRequestID := xid.New().String()
		// Save the search request id.
		l.catalogLock.Lock()
		l.processedSearchRequests[searchRequestID] = struct{}{}
		l.catalogLock.Unlock()
		budgetMap := utils.DistributeBudget(budget, l.network.GetNeighbors())
		for neighbor, budget := range budgetMap {
			// Create the search request.
			msg := types.SearchRequestMessage{
				RequestID: searchRequestID,
				Origin:    l.GetAddress(),
				Pattern:   pattern.String(),
				Budget:    budget,
			}
			transpMsg, _ := l.config.MessageRegistry.MarshalMessage(msg)
			// Send the search request.
			utils.PrintDebug("data", l.GetAddress(), "is unicasting a search request to", neighbor, "in search first.")
			err := l.network.Unicast(neighbor, transpMsg)
			if err != nil {
				utils.PrintDebug("data", "could not unicast the search request:", err.Error())
				continue
			}
		}
		// Collect the received responses.
		collectedResponses := l.notification.MultiResponseCollector(searchRequestID, conf.Timeout, -1)
		utils.PrintDebug("data", l.GetAddress(), "has received following search responses during the timeout", collectedResponses)
		// Iterate through all the received responses within the timeout.
		for _, resp := range collectedResponses {
			searchResp := resp.(*types.SearchReplyMessage)
			for _, fileInfo := range searchResp.Responses {
				if utils.IsFullMatch(fileInfo.Chunks) {
					return fileInfo.Name, nil
				}
			}
		}
		// Otherwise, increase the budget.
		budget = budget * conf.Factor
	}
	return "", nil
}

package utils

import (
	"sync"
	"time"

	"go.dedis.ch/cs438/types"
)

type MultiPacketCallback = func([]types.Message)
type SinglePacketCallback = func(types.Message)

type Cache struct {
	sync.Mutex
	cache        map[string][]types.Message
	collectedIDs map[string]struct{}
}

func NewCache() *Cache {
	return &Cache{
		cache:        map[string][]types.Message{},
		collectedIDs: map[string]struct{}{},
	}
}

func (c *Cache) Save(id string, reply types.Message) {
	c.Lock()
	defer c.Unlock()
	_, ok := c.cache[id]
	if !ok {
		c.cache[id] = []types.Message{}
	}
	c.cache[id] = append(c.cache[id], reply)
}

// Collect returns the cached responses pertaining to the given id. If amount < 0, all the responses are returned.
func (c *Cache) Collect(id string, amount int) []types.Message {
	c.Lock()
	defer c.Unlock()
	l, ok := c.cache[id]
	if !ok {
		return nil
	}
	max := amount
	if max > len(l) {
		max = len(l)
	}
	if amount < 0 {
		max = len(l)
	}
	ret := l[:max]
	delete(c.cache, id)
	return ret
}

type AsyncNotificationHandler struct {
	sync.RWMutex
	waitingChannels map[string]chan types.Message
	cache           *Cache
}

func NewAsyncNotificationHandler() *AsyncNotificationHandler {
	return &AsyncNotificationHandler{
		waitingChannels: make(map[string]chan types.Message),
		cache:           NewCache(),
	}
}

// MultiResponseCollector collects threshold many responses within the given interval. If threshold is set to -1,
// it will collect as many responses as possible within the given interval.
func (a *AsyncNotificationHandler) MultiResponseCollector(id string, interval time.Duration, threshold int) []types.Message {
	// Prepare for dispatches.
	a.Lock()
	bufferSize := threshold
	if threshold < 0 {
		bufferSize = 100
	}
	respChan := make(chan types.Message, bufferSize)
	a.waitingChannels[id] = respChan
	respList := a.cache.Collect(id, threshold)
	a.Unlock()
	// Start collecting replies.
	delay := time.NewTimer(interval)
out:
	for threshold < 0 || len(respList) < threshold {
		select {
		case resp := <-respChan:
			respList = append(respList, resp)
		case <-delay.C:
			break out
		}
	}
	// Cleanup
	a.Lock()
	delete(a.waitingChannels, id)
	a.Unlock()
	return respList
}

// ResponseCollector waits for a response for the given interval. Returns nil in case of time out.
func (a *AsyncNotificationHandler) ResponseCollector(id string, interval time.Duration) types.Message {
	responses := a.MultiResponseCollector(id, interval, 1)
	if len(responses) < 1 {
		return nil
	}
	return responses[0]
}

// DispatchResponse returns whether the given id corresponds to a waiting routine. If not, then the reply is cached.
func (a *AsyncNotificationHandler) DispatchResponse(id string, reply types.Message) bool {
	a.Lock()
	defer a.Unlock()
	ch, ok := a.waitingChannels[id]
	if !ok {
		a.cache.Save(id, reply)
		return false
	}
	select {
	case ch <- reply:
		return true
	default:
		a.cache.Save(id, reply)
	}
	return false
}

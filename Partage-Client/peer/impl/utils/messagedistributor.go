package utils

import (
	"go.dedis.ch/cs438/registry"
	"go.dedis.ch/cs438/types"
	"sync"
)

type MessageDistributor struct {
	sync.Mutex
	handlerMap map[string][]registry.Exec
}

func (m *MessageDistributor) RegisterMessageCallback(msgType types.Message, exec registry.Exec) {
	m.Lock()
	defer m.Unlock()
	_, ok := m.handlerMap[msgType.Name()]
	if !ok {
		m.handlerMap[msgType.Name()] = []registry.Exec{}
	}
	m.handlerMap[msgType.Name()] = append(m.handlerMap[msgType.Name()], exec)
}

func (m *MessageDistributor) HandleMessage(msg types.Message) {

}

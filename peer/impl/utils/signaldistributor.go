package utils

import "sync"

type SignalDistributor struct {
	sync.Mutex
	signalReceiver  chan bool
	signalListeners map[string]chan bool
}

func NewSignalDistributor(receiver chan bool) *SignalDistributor {
	return &SignalDistributor{
		signalReceiver:  receiver,
		signalListeners: make(map[string]chan bool),
	}
}

func (s *SignalDistributor) NewListener(id string) chan bool {
	s.Lock()
	defer s.Unlock()
	c := make(chan bool)
	s.signalListeners[id] = c
	return c
}

func (s *SignalDistributor) GetListener(id string) (chan bool, bool) {
	s.Lock()
	defer s.Unlock()
	c, ok := s.signalListeners[id]
	return c, ok
}

func (s *SignalDistributor) SingleRun() {
	signal := <-s.signalReceiver
	s.Lock()
	defer s.Unlock()
	for _, listener := range s.signalListeners {
		listener <- signal
	}
}

package utils

import "sync"

type SectionLock struct {
	lockMap map[string]*sync.Mutex
}

func NewSectionLock() *SectionLock {
	return &SectionLock{
		lockMap: make(map[string]*sync.Mutex),
	}
}

func (s *SectionLock) Register(id string) {
	s.lockMap[id] = &sync.Mutex{}
}

func (s *SectionLock) Get(id string) *sync.Mutex {
	return s.lockMap[id]
}

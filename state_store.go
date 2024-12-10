package main

import (
	"sync"
	"time"
)

type StateStore struct {
	states          sync.Map
	cleanupInterval time.Duration
}

type stateEntry struct {
	expiresAt time.Time
}

func NewStateStore(cleanupInterval time.Duration) *StateStore {
	ss := &StateStore{
		cleanupInterval: cleanupInterval,
	}
	go ss.periodicCleanup()
	return ss
}

func (s *StateStore) periodicCleanup() {
	ticker := time.NewTicker(s.cleanupInterval)
	for range ticker.C {
		now := time.Now()
		s.states.Range(func(key, value interface{}) bool {
			if entry, ok := value.(stateEntry); ok {
				if now.After(entry.expiresAt) {
					s.states.Delete(key)
				}
			}
			return true
		})
	}
}

func (s *StateStore) StoreState(state string, expiration time.Duration) {
	s.states.Store(state, stateEntry{
		expiresAt: time.Now().Add(expiration),
	})
}

func (s *StateStore) ValidateAndDeleteState(state string) bool {
	if value, ok := s.states.LoadAndDelete(state); ok {
		entry := value.(stateEntry)
		return !time.Now().After(entry.expiresAt)
	}
	return false
}

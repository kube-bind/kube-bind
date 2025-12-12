package session

import (
	"fmt"
	"sync"
)

var ErrSessionNotFound = fmt.Errorf("session not found")

type Store interface {
	Save(state *State) error
	Load(sessionID string) (*State, error)
	Delete(sessionID string) error
}

type InMemoryStore struct {
	lock     sync.Mutex
	sessions map[string]*State
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		lock:     sync.Mutex{},
		sessions: make(map[string]*State),
	}
}

func (s *InMemoryStore) Save(state *State) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sessions[state.SessionID] = state
	return nil
}

func (s *InMemoryStore) Load(sessionID string) (*State, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	state, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return state, nil
}

func (s *InMemoryStore) Delete(sessionID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

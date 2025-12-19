/*
Copyright 2025 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	lock     sync.RWMutex
	sessions map[string]*State
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
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
	s.lock.RLock()
	defer s.lock.RUnlock()
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

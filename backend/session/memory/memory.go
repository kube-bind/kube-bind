/*
Copyright 2026 The Kube Bind Authors.

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

package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kube-bind/kube-bind/backend/session"
)

type pkceEntry struct {
	verifier  string
	expiresAt time.Time
}

type Store struct {
	lock          sync.RWMutex
	sessions      map[string]*session.State
	pkceVerifiers map[string]pkceEntry
}

func New() session.Store {
	return &Store{
		sessions:      make(map[string]*session.State),
		pkceVerifiers: make(map[string]pkceEntry),
	}
}

func (s *Store) Save(ctx context.Context, state *session.State) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sessions[state.SessionID] = state
	return nil
}

func (s *Store) Load(ctx context.Context, sessionID string) (*session.State, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	state, exists := s.sessions[sessionID]
	if !exists {
		return nil, session.ErrSessionNotFound
	}
	return state, nil
}

func (s *Store) Delete(ctx context.Context, sessionID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *Store) SavePKCEVerifier(ctx context.Context, sessionID, verifier string) error {
	if sessionID == "" || verifier == "" {
		return errors.New("sessionID and verifier cannot be empty")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pkceVerifiers[sessionID] = pkceEntry{
		verifier:  verifier,
		expiresAt: time.Now().Add(session.PKCEVerifierTTL),
	}
	return nil
}

func (s *Store) LoadAndDeletePKCEVerifier(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		return "", session.ErrPKCEVerifierNotFound
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	entry, ok := s.pkceVerifiers[sessionID]
	if !ok {
		return "", session.ErrPKCEVerifierNotFound
	}

	delete(s.pkceVerifiers, sessionID)

	if time.Now().After(entry.expiresAt) {
		return "", session.ErrPKCEVerifierNotFound
	}

	return entry.verifier, nil
}

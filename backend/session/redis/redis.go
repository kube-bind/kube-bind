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

package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v4"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/session"
)

type store struct {
	client *goredis.Client
}

func New(redisAddr string, redisPassword string) session.Store {
	client := goredis.NewClient(&goredis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	return &store{
		client: client,
	}
}

func (s *store) Save(ctx context.Context, state *session.State) error {
	encoded, err := state.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}

	key := fmt.Sprintf("session:%s", state.SessionID)

	var ttl time.Duration
	if !state.ExpiresAt.IsZero() {
		ttl = time.Until(state.ExpiresAt)
		if ttl <= 0 {
			klog.FromContext(context.Background()).V(4).Info("Session already expired, skipping saving to redis", "sessionID", state.SessionID)
			return nil
		}
	}

	err = s.client.Set(ctx, key, encoded, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save session to redis: %w", err)
	}
	return nil
}

func (s *store) Load(ctx context.Context, sessionID string) (*session.State, error) {
	key := fmt.Sprintf("session:%s", sessionID)

	val, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, session.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to load session from redis: %w", err)
	}

	var state session.State
	err = msgpack.Unmarshal(val, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to decode state from redis: %w", err)
	}

	return &state, nil
}

func (s *store) Delete(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	err := s.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session from redis: %w", err)
	}
	return nil
}

func (s *store) SavePKCEVerifier(ctx context.Context, sessionID, verifier string) error {
	if sessionID == "" || verifier == "" {
		return errors.New("sessionID and verifier cannot be empty")
	}

	key := fmt.Sprintf("pkce:%s", sessionID)

	err := s.client.Set(ctx, key, verifier, 10*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to save pkce to redis: %w", err)
	}
	return nil
}

func (s *store) LoadAndDeletePKCEVerifier(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		return "", session.ErrPKCEVerifierNotFound
	}

	key := fmt.Sprintf("pkce:%s", sessionID)

	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", session.ErrPKCEVerifierNotFound
		}
		return "", fmt.Errorf("failed to load pkce from redis: %w", err)
	}

	_ = s.client.Del(ctx, key).Err()

	return val, nil
}

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

package session

import (
	"context"
	"fmt"
	"time"
)

const PKCEVerifierTTL = 10 * time.Minute

var ErrSessionNotFound = fmt.Errorf("session not found")
var ErrPKCEVerifierNotFound = fmt.Errorf("pkce verifier not found")

type Store interface {
	Save(ctx context.Context, state *State) error
	Load(ctx context.Context, sessionID string) (*State, error)
	Delete(ctx context.Context, sessionID string) error
	SavePKCEVerifier(ctx context.Context, sessionID, verifier string) error
	LoadAndDeletePKCEVerifier(ctx context.Context, sessionID string) (string, error)
}

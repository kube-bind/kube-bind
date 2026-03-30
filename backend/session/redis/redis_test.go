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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/kube-bind/kube-bind/backend/session"
)

func TestRedisStore(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	store, err := New(s.Addr(), "")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Save and Load Session", func(t *testing.T) {
		state := &session.State{
			SessionID: "sess-123",
			ClusterID: "clus-456",
		}
		state.SetExpiration(1 * time.Hour)

		err := store.Save(ctx, state)
		require.NoError(t, err)

		loaded, err := store.Load(ctx, "sess-123")
		require.NoError(t, err)
		require.Equal(t, "clus-456", loaded.ClusterID)
	})

	t.Run("Load nonexistent Session", func(t *testing.T) {
		_, err := store.Load(ctx, "sess-nonexistent")
		require.ErrorIs(t, err, session.ErrSessionNotFound)
	})

	t.Run("Delete Session", func(t *testing.T) {
		state := &session.State{
			SessionID: "sess-delete-123",
		}
		err := store.Save(ctx, state)
		require.NoError(t, err)

		err = store.Delete(ctx, "sess-delete-123")
		require.NoError(t, err)

		_, err = store.Load(ctx, "sess-delete-123")
		require.ErrorIs(t, err, session.ErrSessionNotFound)
	})

	t.Run("Save and Load PKCE", func(t *testing.T) {
		err := store.SavePKCEVerifier(ctx, "sess-pkce", "my-verifier")
		require.NoError(t, err)

		val, err := store.LoadAndDeletePKCEVerifier(ctx, "sess-pkce")
		require.NoError(t, err)
		require.Equal(t, "my-verifier", val)

		_, err = store.LoadAndDeletePKCEVerifier(ctx, "sess-pkce")
		require.ErrorIs(t, err, session.ErrPKCEVerifierNotFound)
	})

	t.Run("Save Expired Session should not store", func(t *testing.T) {
		state := &session.State{
			SessionID: "sess-expired",
		}

		state.SetExpiration(-1 * time.Hour)

		err := store.Save(ctx, state)
		require.NoError(t, err)

		_, err = store.Load(ctx, "sess-expired")
		require.ErrorIs(t, err, session.ErrSessionNotFound)
	})
}

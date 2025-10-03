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

package framework

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	dexapi "github.com/dexidp/dex/api/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var dexOnce sync.Once

func StartDex(t testing.TB) {
	t.Helper()

	dexOnce.Do(func() {
		dexConfig := os.Getenv("DEX_CONFIG")
		if dexConfig == "" {
			dexConfig = filepath.Clean(filepath.Join(WorkDir, "..", "hack", "dex-config-dev.yaml"))
		}

		t.Logf("Starting dex with config %q", dexConfig)

		dexCmd := exec.Command(
			"dex",
			"serve",
			dexConfig,
		)

		// Ensures that dex is killed when the process ends.
		dexCmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
			Setpgid:   true,
			Pgid:      0,
		}

		require.NoError(t, dexCmd.Start())
	})

	t.Log("Wait for Dex to be ready")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:5556/dex/.well-known/openid-configuration", nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, wait.ForeverTestTimeout, time.Millisecond*100)
	t.Log("Dex is ready")
}

func CreateDexClient(t testing.TB, addr net.Addr) {
	t.Helper()

	_, port, err := net.SplitHostPort(addr.String())
	require.NoError(t, err)
	conn, err := grpc.NewClient("127.0.0.1:5557", grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	client := dexapi.NewDexClient(conn)

	_, err = client.CreateClient(t.Context(), &dexapi.CreateClientReq{
		Client: &dexapi.Client{
			Id:           "kube-bind-" + port,
			Secret:       "ZXhhbXBsZS1hcHAtc2VjcmV0",
			RedirectUris: []string{fmt.Sprintf("http://%s/callback", addr)},
			Public:       true,
			Name:         "kube-bind on port " + port,
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithDeadline(context.Background(), metav1.Now().Add(10*time.Second))
		defer cancel()
		conn, err := grpc.NewClient("127.0.0.1:5557", grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
		require.NoError(t, err)
		_, err = dexapi.NewDexClient(conn).DeleteClient(ctx, &dexapi.DeleteClientReq{Id: "kube-bind-" + port})
		require.NoError(t, err)
	})
}

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

package plugin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	bindconfig "github.com/kube-bind/kube-bind/cli/pkg/config"
	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// LoginOptions contains the options for the login command
type LoginOptions struct {
	*base.Options
	Logs    *logs.Options
	Streams genericclioptions.IOStreams

	// ShowToken displays the stored token after successful authentication
	ShowToken bool

	// SkipBrowser skips opening the browser automatically.
	SkipBrowser bool

	// Timeout for the authentication flow
	Timeout time.Duration

	loginClient *http.Client
}

// TokenResponse represents the response from the OAuth callback
// Important: this stuct must match one on backend/auth/types.go
type TokenResponse struct {
	// OAuth2 token fields
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Error        string    `json:"error,omitempty"`
	ErrorMessage string    `json:"error_description,omitempty"`

	Cluster string `json:"cluster,omitempty"`
}

// NewLoginOptions creates a new LoginOptions
func NewLoginOptions(streams genericclioptions.IOStreams) *LoginOptions {
	opts := base.NewOptions(streams)
	return &LoginOptions{
		Options: opts,
		Logs:    logs.NewOptions(),
		Streams: streams,
		Timeout: 5 * time.Minute,
	}
}

// AddCmdFlags adds command line flags
func (o *LoginOptions) AddCmdFlags(cmd *cobra.Command) {
	o.Options.BindFlags(cmd)
	logsv1.AddFlags(o.Logs, cmd.Flags())

	cmd.Flags().BoolVar(&o.ShowToken, "show-token", false, "Display the stored token after successful authentication")
	cmd.Flags().DurationVar(&o.Timeout, "timeout", o.Timeout, "Timeout for the authentication flow")
	cmd.Flags().BoolVarP(&o.SkipBrowser, "skip-browser", "", false, "Skip opening the browser automatically")
}

// Complete completes the options
func (o *LoginOptions) Complete(args []string) error {
	if len(args) > 0 {
		o.Options.ServerName = strings.TrimSuffix(args[0], "/")
	}
	err := o.Options.Complete(true)
	if err != nil {
		return err
	}

	o.loginClient = http.DefaultClient

	return nil
}

// Validate validates the options
func (o *LoginOptions) Validate() error {
	return o.Options.Validate()
}

// Run executes the login command
func (o *LoginOptions) Run(ctx context.Context, authURLCh chan<- string) error {
	config := o.Options.GetConfig()

	// Generate a random session ID for cli session to verify callback requests.
	sessionID := rand.Text()

	// Setup callback server with random port
	tokenCh := make(chan *TokenResponse, 1)
	errCh := make(chan error, 1)

	// Get provider information
	fmt.Fprintf(o.Streams.ErrOut, "Connecting to kube-bind server %s...\n", o.Options.ServerName)
	provider, err := o.getProvider(ctx)
	if err != nil {
		return fmt.Errorf("failed to get provider information: %w", err)
	}

	server, localCallbackURL, err := o.startCallbackServerWithRandomPort(tokenCh, errCh)
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer server.Close()
	fmt.Fprintf(o.Streams.ErrOut, "Started local callback server at %s\n", localCallbackURL)

	// Start authentication flow
	authURL, err := o.buildAuthURL(provider, localCallbackURL, sessionID)
	if err != nil {
		return fmt.Errorf("failed to build auth URL: %w", err)
	}
	if authURLCh != nil {
		authURLCh <- authURL
	}

	if !o.SkipBrowser {
		fmt.Fprintf(o.Streams.ErrOut, "Opening browser for authentication... \n")
		err = base.OpenBrowser(authURL)
		if err != nil {
			fmt.Fprintf(o.Streams.ErrOut, "Failed to open browser automatically: %v\n", err)
			fmt.Fprintf(o.Streams.ErrOut, "Please manually open: %s\n\n", authURL)
		}
	} else {
		fmt.Fprintf(o.Streams.ErrOut, "Please open the following URL in your browser to authenticate:\n%s\n\n", authURL)
	}

	// Wait for callback with timeout
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	var token *TokenResponse
	select {
	case token = <-tokenCh:
		if token.Error != "" {
			return fmt.Errorf("authentication failed: %s - %s", token.Error, token.ErrorMessage)
		}
	case err := <-errCh:
		return fmt.Errorf("callback server error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("authentication timed out after %v", o.Timeout)
	}

	// Calculate expiration time
	if token.ExpiresIn > 0 && token.ExpiresAt.IsZero() {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	serverHost, err := url.Parse(o.Options.ServerName)
	if err != nil {
		return fmt.Errorf("failed to parse server URL: %w", err)
	}

	// Store token in config
	serverConfig := &bindconfig.Server{
		URL:         fmt.Sprintf("%s://%s", serverHost.Scheme, serverHost.Host),
		Cluster:     token.Cluster,
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresAt:   token.ExpiresAt,
	}

	// Use cluster-aware server key
	serverURL := fmt.Sprintf("%s://%s", serverHost.Scheme, serverHost.Host)
	config.AddServerWithCluster(serverURL, token.Cluster, serverConfig)
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Set this as the current server using the cluster-aware key
	if err := config.SetCurrentServer(serverURL, token.Cluster); err != nil {
		return fmt.Errorf("failed to set current server: %w", err)
	}

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save current server: %w", err)
	}

	if token.Cluster != "" {
		fmt.Fprintf(o.Streams.ErrOut, "🔑 Successfully authenticated to %s (cluster: %s)\n", serverHost.Host, token.Cluster)
		fmt.Fprintf(o.Streams.ErrOut, "   Server key: %s\n", fmt.Sprintf("%s@%s", serverURL, token.Cluster))
	} else {
		fmt.Fprintf(o.Streams.ErrOut, "🔑 Successfully authenticated to %s\n", serverHost.Host)
	}

	if o.ShowToken {
		displayToken := token.AccessToken
		if len(displayToken) > 20 {
			displayToken = displayToken[:20] + "..."
		}
		fmt.Fprintf(o.Streams.ErrOut, "\nStored token: %s\n", displayToken)
	}

	configPath, _ := config.GetConfigPath()
	fmt.Fprintf(o.Streams.ErrOut, "Configuration saved to: %s\n", configPath)

	return nil
}

func (o *LoginOptions) getProvider(ctx context.Context) (*kubebindv1alpha2.BindingProvider, error) {
	url, err := url.Parse(o.Options.ServerName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %w", err)
	}

	if !strings.Contains(url.Path, "/api/exports") {
		url.Path = "/api/exports"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.loginClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var provider kubebindv1alpha2.BindingProvider
	if err := json.Unmarshal(body, &provider); err != nil {
		return nil, err
	}

	return &provider, nil
}

func (o *LoginOptions) buildAuthURL(provider *kubebindv1alpha2.BindingProvider, redirectURL, sessionID string) (string, error) {
	var oauth2Method *kubebindv1alpha2.OAuth2CodeGrant
	for _, m := range provider.AuthenticationMethods {
		if m.Method == "OAuth2CodeGrant" {
			oauth2Method = m.OAuth2CodeGrant
			break
		}
	}

	if oauth2Method == nil {
		return "", fmt.Errorf("server does not support OAuth2 code grant flow")
	}

	u, err := url.Parse(oauth2Method.AuthenticatedURL)
	if err != nil {
		return "", err
	}

	values := u.Query()
	values.Set("redirect_url", redirectURL)
	values.Set("session_id", sessionID)
	values.Set("client_type", "cli")
	if o.ClusterName != "" {
		values.Set("cluster_id", o.ClusterName)
	}
	u.RawQuery = values.Encode()

	return u.String(), nil
}

func (o *LoginOptions) startCallbackServerWithRandomPort(tokenCh chan<- *TokenResponse, errCh chan<- error) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("failed to find available port: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Setup HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := &TokenResponse{
			Error:        r.URL.Query().Get("error"),
			ErrorMessage: r.URL.Query().Get("error_description"),
			AccessToken:  r.URL.Query().Get("access_token"),
			TokenType:    r.URL.Query().Get("token_type"),
			Cluster:      r.URL.Query().Get("cluster_id"),
		}

		if expiresIn := r.URL.Query().Get("expires_in"); expiresIn != "" {
			if exp, err := strconv.ParseInt(expiresIn, 10, 64); err == nil {
				token.ExpiresIn = exp
			}
		}

		// Send success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Kube-Bind Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 100px; }
        .success { color: green; }
        .error { color: red; }
    </style>
</head>
<body>
    <h1>Kube-Bind Authentication</h1>
    <div class="%s">
        <h2>%s</h2>
        <p>You can now close this window and return to the CLI.</p>
    </div>
</body>
</html>`,
			map[bool]string{true: "success", false: "error"}[token.Error == ""],
			map[bool]string{true: "Authentication Successful!", false: "Authentication Failed"}[token.Error == ""])

		select {
		case tokenCh <- token:
		default:
		}
	})

	server := &http.Server{
		ReadTimeout: time.Minute * 5,
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	return server, callbackURL, nil
}

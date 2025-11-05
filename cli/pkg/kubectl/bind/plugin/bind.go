/*
Copyright 2022 The Kube Bind Authors.

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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	bindapiservice "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/plugin"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// BindResult represents the result from the UI callback
type BindResult struct {
	Success          bool   `json:"success"`
	Message          string `json:"message,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`

	Response kubebindv1alpha2.BindingResourceResponse `json:"response,omitempty"`
}

// BindOptions contains the options for creating an APIBinding.
type BindOptions struct {
	*base.Options
	Logs *logs.Options

	Print   *genericclioptions.PrintFlags
	printer printers.ResourcePrinter
	DryRun  bool

	// skipKonnector skips the deployment of the konnector.
	SkipKonnector bool

	// The konnector image to use and override default konnector image
	KonnectorImageOverride string

	// Runner is runs the command. It can be replaced in tests.
	Runner func(cmd *exec.Cmd) error

	flags *pflag.FlagSet
}

// NewBindOptions returns new BindOptions.
func NewBindOptions(streams genericclioptions.IOStreams) *BindOptions {
	opts := &BindOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("kubectl-bind").WithDefaultOutput("yaml"),

		Runner: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
	}

	return opts
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *BindOptions) AddCmdFlags(cmd *cobra.Command) {
	b.flags = cmd.Flags()

	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)

	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", b.SkipKonnector, "Skip the deployment of the konnector")
	cmd.Flags().BoolVarP(&b.DryRun, "dry-run", "d", b.DryRun, "If true, only print the requests that would be sent to the service provider after authentication, without actually binding.")
	cmd.Flags().StringVar(&b.KonnectorImageOverride, "konnector-image", b.KonnectorImageOverride, "The konnector image to use")
}

// Complete ensures all fields are initialized.
func (b *BindOptions) Complete(args []string) error {
	// Try base completion, but don't fail if no current server is configured
	if err := b.Options.Complete(false); err != nil {
		return err
	}

	printer, err := b.Print.ToPrinter()
	if err != nil {
		return err
	}

	b.printer = printer

	return nil
}

// Validate validates the BindOptions are complete and usable.
func (b *BindOptions) Validate() error {
	if b.ServerName == "" {
		return fmt.Errorf("server is required")
	}

	if _, err := url.Parse(b.ServerName); err != nil {
		return fmt.Errorf("invalid url %q: %w", b.ServerName, err)
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindOptions) Run(ctx context.Context, urlCh chan<- string) error {
	// Always use UI mode with callback listener
	return b.runWithCallback(ctx, urlCh)
}

// runWithCallback creates a local callback listener and opens the UI
func (b *BindOptions) runWithCallback(ctx context.Context, _ chan<- string) error {
	_, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get client config: %w", err)
	}

	// Generate session ID. It is used to verify callback.
	sessionID := rand.Text()

	// Setup callback server with random port
	resultCh := make(chan *BindResult, 1)
	errCh := make(chan error, 1)

	callbackServer, callbackPort, err := b.startCallbackServer(resultCh, errCh, sessionID)
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer callbackServer.Close()

	// Build the UI URL with callback parameters
	uiURL, err := b.buildUIURL(callbackPort, sessionID, b.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to build UI URL: %w", err)
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "🌐 Opening kube-bind UI in your browser...\n")
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "    %s\n\n", uiURL)

	// Open browser
	if err := base.OpenBrowser(uiURL); err != nil {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Failed to open browser automatically: %v\n", err)
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Please manually open: %s\n\n", uiURL)
	} else {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Browser opened successfully\n")
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "Waiting for binding completion from UI...\n")
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "   (Press Ctrl+C to cancel)\n\n")

	// Wait for callback result with context cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	select {
	case result := <-resultCh:
		if result.Error != "" {
			return fmt.Errorf("binding failed: %s - %s", result.Error, result.ErrorDescription)
		}

		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Binding completed successfully!\n")
		if result.Message != "" {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "   %s\n", result.Message)
		}

		// Handle dry-run mode
		if b.DryRun {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "Dry-run mode: outputting APIServiceExport requests\n\n")

			// Print each request from the response using the configured printer
			for i, request := range result.Response.Requests {
				if i > 0 && b.Print.OutputFormat != nil && *b.Print.OutputFormat == "yaml" {
					fmt.Fprintf(b.Options.IOStreams.Out, "---\n")
				}

				// TODO: support proper k/k style printers.
				// Unmarshal the raw JSON into an APIServiceExportRequest
				var apiRequest kubebindv1alpha2.APIServiceExportRequest
				if err := json.Unmarshal(request.Raw, &apiRequest); err != nil {
					return fmt.Errorf("failed to unmarshal request %d: %w", i, err)
				}

				// Use the printer to output in the requested format
				if b.printer != nil {
					if err := b.printer.PrintObj(&apiRequest, b.Options.IOStreams.Out); err != nil {
						return fmt.Errorf("failed to print request %d: %w", i, err)
					}
				} else {
					// Fallback to raw JSON output if printer is not available
					fmt.Fprintf(b.Options.IOStreams.Out, "%s", request.Raw)
				}
			}
		}

		// Create bindings using the shared binder
		config, err := b.Options.ClientConfig.ClientConfig()
		if err != nil {
			return fmt.Errorf("failed to get client config: %w", err)
		}

		bindings, err := b.bindResponseToAPIServiceBindings(ctx, config, &result.Response)
		if err != nil {
			return fmt.Errorf("failed to create APIServiceBindings: %w", err)
		}

		if b.DryRun {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "\nDry-run mode: no APIServiceBindings were created.\n")
			return nil
		}

		// Print the results
		if len(bindings) > 0 {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "Created %d APIServiceBinding(s):\n", len(bindings))
			for _, binding := range bindings {
				fmt.Fprintf(b.Options.IOStreams.Out, "  - %s\n", binding.Name)
			}
		}

		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Resources bound successfully!\n")
		return nil

	case err := <-errCh:
		return fmt.Errorf("callback server error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("operation cancelled")
	}
}

// buildUIURL constructs the UI URL with callback parameters
func (b *BindOptions) buildUIURL(callbackPort int, sessionID, clusterID string) (string, error) {
	// Parse the base URL
	u, err := url.Parse(b.ServerName)
	if err != nil {
		return "", fmt.Errorf("invalid server URL: %w", err)
	}

	redirectURL := "http://127.0.0.1:" + strconv.Itoa(callbackPort) + "/callback"

	// Add query parameters
	values := u.Query()
	values.Add("session_id", sessionID)
	values.Add("redirect_url", redirectURL)
	if clusterID != "" {
		values.Add("cluster_id", clusterID)
	}
	u.RawQuery = values.Encode()

	return u.String(), nil
}

// startCallbackServer starts a local HTTP server to receive the callback from the UI
func (b *BindOptions) startCallbackServer(resultCh chan<- *BindResult, errCh chan<- error, sessionID string) (*http.Server, int, error) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find available port: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Setup HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Validate session ID
		if r.URL.Query().Get("session_id") != sessionID {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		result := &BindResult{}
		payload := kubebindv1alpha2.BindingResourceResponse{}

		// Parse query parameters (for simple callbacks)
		query := r.URL.Query()

		response := query.Get("binding_response")
		responseData, err := base64.URLEncoding.DecodeString(response)
		if err != nil {
			http.Error(w, "Invalid binding_response", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(responseData, &payload); err != nil {
			http.Error(w, "Invalid binding_response JSON", http.StatusBadRequest)
			return
		}

		result.Response = payload

		result.Success = query.Get("success") == "true"
		result.Message = query.Get("message")
		result.Error = query.Get("error")
		result.ErrorDescription = query.Get("error_description")

		// Send success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Kube-Bind - Binding Complete</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 100px; }
        .success { color: green; }
        .error { color: red; }
    </style>
</head>
<body>
    <h1>Kube-Bind</h1>
    <div class="%s">
        <h2>%s</h2>
        <p>You can now close this window and return to the CLI.</p>
    </div>
</body>
</html>`,
			map[bool]string{true: "success", false: "error"}[result.Success || result.Error == ""],
			map[bool]string{true: "Binding Completed Successfully!", false: "Binding Failed"}[result.Success || result.Error == ""])

		// Send result to channel
		select {
		case resultCh <- result:
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

	return server, port, nil
}

// bindResponseToAPIServiceBindings uses the shared binder to create API service bindings
func (b *BindOptions) bindResponseToAPIServiceBindings(ctx context.Context, config *rest.Config, response *kubebindv1alpha2.BindingResourceResponse) ([]*kubebindv1alpha2.APIServiceBinding, error) {
	binderOpts := &bindapiservice.BinderOptions{
		IOStreams:              b.Options.IOStreams,
		SkipKonnector:          b.SkipKonnector,
		KonnectorImageOverride: b.KonnectorImageOverride,
		DryRun:                 b.DryRun,
	}

	binder := bindapiservice.NewBinder(config, binderOpts)
	return binder.BindFromResponse(ctx, response)
}

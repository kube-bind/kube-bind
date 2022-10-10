/*
Copyright 2022 The Kubectl Bind contributors.

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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/authenticator"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
)

// BindOptions contains the options for creating an APIBinding.
type BindOptions struct {
	*base.Options

	// url is the argument accepted by the command. It contains the
	// reference to where an APIService exists.
	url string
}

// NewBindOptions returns new BindOptions.
func NewBindOptions(streams genericclioptions.IOStreams) *BindOptions {
	return &BindOptions{
		Options: base.NewOptions(streams),
	}
}

// BindFlags binds fields to cmd's flagset.
func (b *BindOptions) BindFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
}

// Complete ensures all fields are initialized.
func (b *BindOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.url = args[0]
	}
	return nil
}

// Validate validates the BindOptions are complete and usable.
func (b *BindOptions) Validate() error {
	if b.url == "" {
		return errors.New("url is required as an argument") // should not happen because we validate that before
	}

	if _, err := url.Parse(b.url); err != nil {
		return fmt.Errorf("invalid url %q: %w", b.url, err)
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindOptions) Run(ctx context.Context) error {
	_, err := b.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	sessionID := rand.String(rand.IntnRange(10, 15))

	auth, err := authenticator.NewDefaultAuthenticator(1*time.Minute, b.serviceBinding)
	if err != nil {
		return err
	}

	url, err := url.Parse(b.url)
	if err != nil {
		return err // should never happen because we test this in Validate()
	}

	values := url.Query()
	values.Add("redirect_url", auth.Endpoint(ctx))
	values.Add("session_id", sessionID)

	url.RawQuery = values.Encode()
	resp, err := http.Get(url.String())
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", url, err)
	}

	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("failed to close body: %v", err)
	}

	if err := auth.Execute(ctx); err != nil {
		return err
	}

	return nil
}

func (b *BindOptions) serviceBinding(ctx context.Context) error {
	body, err := io.ReadAll(nil)
	if err != nil {
		return fmt.Errorf("failed to read body from %s: %w", b.url, err)
	}

	var obj metav1.PartialObjectMetadata
	if err := yaml.Unmarshal(body, &obj); err != nil {
		return fmt.Errorf("failed to unmashal response from %s (%q): %w", b.url, strings.ReplaceAll(string(body[:40]), "\n", "\\n"), err)
	}

	// TODO: generalize this, with scheme, and potentially forking into kubectl-bind-<lower(kind)>
	if obj.APIVersion != "kube-bind.io/v1alpha1" {
		return fmt.Errorf("expected apiVersion kube-bind.io/v1alpha1, got %q", obj.APIVersion)
	}

	if obj.Kind != "APIService" {
		return fmt.Errorf("expected kind APIService, got %q", obj.Kind)
	}

	var apiService kubebindv1alpha1.APIService
	if err := yaml.Unmarshal(body, &apiService); err != nil {
		return fmt.Errorf("failed to unmashal response from %s as APIService: %w", b.url, err)
	}

	fmt.Fprint(b.Out, string(body))

	return nil
}

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
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/kube-bind/kube-bind/pkg/authenticator"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin/resources"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeclient "k8s.io/client-go/kubernetes"
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

	auth, err := authenticator.NewDefaultAuthenticator(1*time.Minute, b.serviceBinding)
	if err != nil {
		return err
	}

	exportURL, err := url.Parse(b.url)
	if err != nil {
		return err // should never happen because we test this in Validate()
	}

	authURL, err := fetchAuthenticationRoute(exportURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch authentication url: %v", err)
	}

	parsedAuthURL, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("failed to parse auth url: %v", err)
	}

	sessionID := rand.String(rand.IntnRange(20, 30))

	if err := authenticate(parsedAuthURL, auth.Endpoint(ctx), sessionID); err != nil {
		return err
	}

	if err := auth.Execute(ctx); err != nil {
		return err
	}

	return nil
}

func (b *BindOptions) serviceBinding(clusterName, sessionID, kubeconfig, accessToken string) error {
	config, err := b.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	client, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}

	namespace, _, err := b.ClientConfig.Namespace()
	if err != nil {
		return err
	}

	authSecret, err := resources.EnsureServiceBindingAuthData(context.TODO(),
		clusterName, kubeconfig, accessToken, namespace, client)
	if err != nil {
		return err
	}

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = resources.EnsureServiceBindingSession(context.TODO(),
		clusterName, authSecret, sessionID, namespace, bindClient)
	if err != nil {
		return err
	}

	return nil
}

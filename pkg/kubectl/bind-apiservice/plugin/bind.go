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
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/kube-bind/kube-bind/deploy/konnector"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
)

// BindAPIServiceOptions are the options for the kubectl-bind-apiservice command.
type BindAPIServiceOptions struct {
	Base *base.Options
	Logs *logs.Options

	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	OutputFormat       string
	DryRun             bool

	token     string
	tokenFile string
	file      string

	// url is the argument accepted by the command
	// reference to where an  exists.
	url string
}

// NewBindAPIServiceOptions returns new BindAPIServiceOptions.
func NewBindAPIServiceOptions(streams genericclioptions.IOStreams) *BindAPIServiceOptions {
	return &BindAPIServiceOptions{
		Base: base.NewOptions(streams),
		Logs: logs.NewOptions(),
	}
}

// BindFlags binds fields to cmd's flagset.
func (b *BindAPIServiceOptions) BindFlags(cmd *cobra.Command) {
	b.Base.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())

	cmd.Flags().StringVar(&b.token, "token", b.token, "The bearer token to use to authenticate for a APIService binding request")
	cmd.Flags().StringVar(&b.token, "token-file", b.token, "A file with the bearer token to use to authenticate for a APIService binding request")
	cmd.Flags().StringVarP(&b.token, "file", "f", b.token, "The bearer token to use to authenticate for a APIService binding request. Use - to read from stdin")
}

// Complete ensures all fields are initialized.
func (b *BindAPIServiceOptions) Complete(args []string) error {
	if err := b.Base.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.url = args[0]
	}
	return nil
}

// Validate validates the BindAPIServiceOptions are complete and usable.
func (b *BindAPIServiceOptions) Validate() error {
	if b.url == "" && b.file == "" {
		return errors.New("url or file is required")
	}
	if b.url != "" && b.file != "" {
		return errors.New("url and file are mutually exclusive")
	}
	if b.url != "" {
		if _, err := url.Parse(b.url); err != nil {
			return fmt.Errorf("invalid url %q: %w", b.url, err)
		}
	}

	if b.token == "" && b.tokenFile == "" {
		return errors.New("token or token-file is required")
	}
	if b.token != "" && b.tokenFile != "" {
		return errors.New("token and token-file are mutually exclusive")
	}

	return b.Base.Validate()
}

// Run starts the binding process.
func (b *BindAPIServiceOptions) Run(ctx context.Context) error {
	// nolint: staticcheck
	cfg, err := b.Base.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	if b.tokenFile != "" {
		bs, err := os.ReadFile(b.tokenFile)
		if err != nil {
			return fmt.Errorf("failed to read token file %s: %w", b.tokenFile, err)
		}
		b.token = string(bs)
	}

	bs, err := b.getRequestManifest()
	if err != nil {
		return err
	}
	request, err := b.unmarshalManifest(bs)
	if err != nil {
		return err
	}
	result, err := b.postRequestManifest(request)
	if err != nil {
		return err
	}
	if err := b.deployKonnector(ctx, cfg); err != nil {
		return err
	}
	if err := b.createAPIServiceBindings(ctx, result, cfg); err != nil {
		return err
	}

	return nil
}

func (b *BindAPIServiceOptions) getRequestManifest() ([]byte, error) {
	if b.url != "" {
		resp, err := http.Get(b.url)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", b.url, err)
		}
		defer resp.Body.Close() // nolint: errcheck
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return body, nil
	} else if b.file == "-" {
		body, err := io.ReadAll(b.Base.IOStreams.In)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return body, nil
	}

	body, err := os.ReadFile(b.file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", b.file, err)
	}
	return body, nil
}

func (b *BindAPIServiceOptions) unmarshalManifest(bs []byte) (*kubebindv1alpha1.APIServiceBindingRequest, error) {
	var request kubebindv1alpha1.APIServiceBindingRequest
	if err := yaml.Unmarshal(bs, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	if request.APIVersion != kubebindv1alpha1.SchemeGroupVersion.String() {
		return nil, fmt.Errorf("invalid apiVersion %q", request.APIVersion)
	}
	if request.Kind != "APIServiceBindingRequest" {
		return nil, fmt.Errorf("invalid kind %q", request.Kind)
	}
	return &request, nil
}

func (b *BindAPIServiceOptions) postRequestManifest(request *kubebindv1alpha1.APIServiceBindingRequest) (*kubebindv1alpha1.APIServiceBindingRequest, error) {
	// TODO(moath): implement posting the request manifest to endpoint
	panic("not implemented")
}

// nolint: unused
func (b *BindAPIServiceOptions) createAPIServiceBindings(ctx context.Context, result *kubebindv1alpha1.APIServiceBindingRequest, cfg *rest.Config) error {
	// TODO(moath): create secret and service bindings
	panic("not implemented")
}

// nolint: unused
func (b *BindAPIServiceOptions) deployKonnector(ctx context.Context, cfg *rest.Config) error {
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	if err := konnector.Bootstrap(ctx, discoveryClient, client, sets.NewString()); err != nil {
		return err
	}

	// TODO: check health
	// TODO: wait for APIServiceBinding API to be available

	return nil
}

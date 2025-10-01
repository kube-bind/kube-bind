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

package backend

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	"github.com/kcp-dev/multicluster-provider/apiexport"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	"github.com/kube-bind/kube-bind/backend/options"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Config struct {
	Options *options.CompletedOptions

	Provider                 multicluster.Provider
	ExternalAddressGenerator kuberesources.ExternalAddreesGeneratorFunc
	Manager                  mcmanager.Manager
	Scheme                   *runtime.Scheme

	ClientConfig *rest.Config
}

func NewConfig(options *options.CompletedOptions) (*Config, error) {
	config := &Config{
		Options: options,
	}

	// create clients
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = options.KubeConfig
	var err error
	config.ClientConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		return nil, err
	}
	config.ClientConfig = rest.CopyConfig(config.ClientConfig)
	config.ClientConfig = rest.AddUserAgent(config.ClientConfig, "kube-bind-backend")

	if options.ServerURL != "" {
		config.ClientConfig.Host = options.ServerURL
	}

	// Set up controller-runtime manager
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding client-go scheme: %w", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding apiextensions scheme: %w", err)
	}
	if err := kubebindv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding kubebind scheme: %w", err)
	}
	if err := kubebindv1alpha2.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding kubebind scheme: %w", err)
	}

	config.Scheme = scheme

	switch options.Provider {
	case "kcp":
		if err := apisv1alpha1.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding apis scheme: %w", err)
		}
		if err := apisv1alpha2.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding apis scheme: %w", err)
		}
		provider, err := apiexport.New(config.ClientConfig, apiexport.Options{
			Scheme: scheme,
		})
		if err != nil {
			return nil, fmt.Errorf("error setting up kcp provider: %w", err)
		}

		config.ExternalAddressGenerator = parseKCPServerURL
		config.Provider = provider
	default:
		config.ExternalAddressGenerator = kuberesources.DefaultExternalAddreesGenerator
		config.Provider = nil
	}

	opts := ctrl.Options{
		Controller: ctrlconfig.Controller{
			SkipNameValidation: ptr.To(config.Options.ExtraOptions.TestingSkipNameValidation),
		},
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Scheme: scheme,
	}

	manager, err := mcmanager.New(config.ClientConfig, config.Provider, opts)
	if err != nil {
		return nil, fmt.Errorf("error setting up controller manager: %w", err)
	}

	config.Manager = manager

	return config, nil
}

func parseKCPServerURL(ctx context.Context, clusterConfig *rest.Config) (string, error) {
	// In kcp case, we are talking via apiexport so clientconfig will be pointing to
	// https://192.168.2.166:6443/services/apiexport/2sssrgg8ivlpw0cy/kube-bind.io/clusters/2p0rtkf7b697s6mj
	// We need to extract host and /clusters/... part
	u, err := url.Parse(clusterConfig.Host)
	if err != nil {
		return "", err
	}

	// Extract cluster ID from the path
	// Path format: /services/apiexport/{export-id}/kube-bind.io/clusters/{cluster-id}
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 5 || pathParts[4] != "clusters" {
		return "", fmt.Errorf("invalid apiexport URL format")
	}

	clusterID := pathParts[5]

	// Construct new URL with cluster path
	u.Path = "/clusters/" + clusterID

	return u.String(), nil
}

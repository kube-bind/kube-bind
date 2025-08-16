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
	"fmt"

	"github.com/kcp-dev/multicluster-provider/apiexport"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	"github.com/kube-bind/kube-bind/contrib/example-backend/options"
)

type Config struct {
	Options *options.CompletedOptions

	Provider multicluster.Provider

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

	switch options.Provider {
	case "kcp":
		provider, err := apiexport.New(config.ClientConfig, apiexport.Options{})
		if err != nil {
			return nil, fmt.Errorf("error setting up kcp provider: %w", err)
		}
		config.Provider = provider
	default:
		config.Provider = nil
	}

	return config, nil
}

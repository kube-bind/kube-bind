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

package base

import (
	"github.com/kube-bind/kube-bind/cli/pkg/client"
)

// GetAuthenticatedClientWithLogin returns an authenticated client for the configured server.
func (o *Options) GetAuthenticatedClient() (client.Client, error) {
	// First try to create an authenticated client
	// Now use authenticated client
	return client.NewClient(*o.server,
		client.WithInsecure(o.SkipInsecure),
	)
}

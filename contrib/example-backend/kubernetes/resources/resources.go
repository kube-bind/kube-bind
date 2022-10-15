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

package resources

const (
	ServiceAccountTokenType       = "kubernetes.io/service-account-token"
	ServiceAccountTokenAnnotation = "kubernetes.io/service-account.name"
	ClusterAdminName              = "cluster-admin"
	ClusterBindingName            = "cluster"
)

// AuthCode represents the data that are needed to complete the full authentication workflow.
// TODO: We should come up with something more standarized but this is good enough for now.
type AuthCode struct {
	RedirectURL string `json:"redirectURL"`
	SessionID   string `json:"sid"`
}

// AuthResponse contains the authentication data which is needed to connect to the service provider
// cluster.
type AuthResponse struct {
	SessionID  string `json:"sid"`
	ID         string `json:"id"`
	Kubeconfig []byte `json:"kubeconfig"`
	Resource   string `json:"resource"`
	Group      string `json:"group"`
	Export     string `json:"export"`
}

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
	ClusterAdminName              = "kubebind-cluster-admin"
	SessionIDs                    = "session-ids"
	ClusterBindingName            = "cluster"
	ClusterBindingKubeConfig      = "cluster-admin-kubeconfig"
)

// AuthCode represents the data that are needed to complete the full authentication workflow.
// TODO: We should come up with something more standarized but this is good enough for now.
type AuthCode struct {
	RedirectURL string
	SessionID   string
}

// AuthResponse contains the authentication data which is needed to connect to the service provider
// cluster.
// TODO: think about replace the cluster name with an identity which represents the cluster instead of the cluster name
// TODO: think about adding a URL of the requested service provider as well to distinguish the service that is being served
type AuthResponse struct {
	SessionID   string
	Kubeconfig  []byte
	ClusterName string
}

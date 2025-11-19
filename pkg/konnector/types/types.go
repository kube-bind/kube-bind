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

package types

// ClusterNamespaceAnnotationKey is the annotation key to identify the
// cluster namespace that any synced object on the provider side belongs to.
// This annotation is set on all synced objects, regardless of scope or
// isolation mode.
const ClusterNamespaceAnnotationKey = "kube-bind.io/cluster-namespace"

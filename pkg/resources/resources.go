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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// IsClaimed returns true if the given object matches the given selector and named resources.
func IsClaimed(selector kubebindv1alpha2.Selector, obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}
	// Empty selector selects everything
	if selector.LabelSelector == nil && len(selector.NamedResources) == 0 {
		return true
	}

	// Both label selector and named resources must match if both are specified
	labelSelectorMatches := true
	namedResourceMatches := true

	// Check label selector if specified
	if selector.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			return false
		}
		l := obj.GetLabels()
		if l == nil {
			l = make(map[string]string)
		}
		labelSelectorMatches = selector.Matches(labels.Set(l))
	}

	// Check named resources if specified
	if len(selector.NamedResources) > 0 {
		namedResourceMatches = false // Default to false, must match at least one
		for _, nr := range selector.NamedResources {
			if nr.Namespace != "" && nr.Namespace != obj.GetNamespace() {
				continue
			}
			if nr.Name == obj.GetName() {
				namedResourceMatches = true
				break
			}
		}
	}

	return labelSelectorMatches && namedResourceMatches
}

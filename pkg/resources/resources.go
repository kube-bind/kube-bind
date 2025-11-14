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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// ServiceNamespaceLister is an interface for listing APIServiceNamespaces
type ServiceNamespaceLister interface {
	List(selector labels.Selector) ([]*kubebindv1alpha2.APIServiceNamespace, error)
}

// IsClaimed returns true if the given object matches the given selector and named resources.
// Logger here is already at V(4) level.
func IsClaimed(logger klog.Logger, selector kubebindv1alpha2.Selector, obj *unstructured.Unstructured, potentiallyReferencedResources *unstructured.UnstructuredList) bool {
	if obj == nil {
		return false
	}
	// Empty selector selects nothing
	if selector.LabelSelector == nil && len(selector.NamedResources) == 0 && len(selector.References) == 0 {
		return false
	}

	// Both label selector and named resources must match if both are specified
	labelSelectorMatches := false
	namedResourceMatches := false
	referenceMatches := false

	// Check label selector if specified
	if selector.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			runtime.HandleError(fmt.Errorf("invalid label selector in permission claim: %w", err))
			return false
		}
		l := obj.GetLabels()
		if l == nil {
			l = make(map[string]string)
		}

		labelSelectorMatches = selector.Matches(labels.Set(l))
		if labelSelectorMatches {
			logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "labelSelector", "selector", selector.String(), "objectLabels", l)
		}
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
				logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "namedResource", "namedResource", nr)
				break
			}
		}
	}

	if len(selector.References) > 0 {
		if potentiallyReferencedResources == nil {
			referenceMatches = false
		} else {
			referenceMatches = false // Default to false, must match at least one
			for _, refObj := range potentiallyReferencedResources.Items {
				// Marshal the referenced object to JSON for JSONPath evaluation
				jsonData, err := refObj.MarshalJSON()
				if err != nil {
					continue // Skip this object if we can't marshal it
				}

				for _, ref := range selector.References {
					selector := ref
					// Extract name and namespace using JSONPath expressions
					nameResult := gjson.Get(string(jsonData), selector.JSONPath.Name)
					var namespaceResult gjson.Result
					if selector.JSONPath.Namespace != "" {
						namespaceResult = gjson.Get(string(jsonData), selector.JSONPath.Namespace)
					}

					// Check if the JSONPath results match the current object
					if nameResult.Exists() {
						// Handle both single values and arrays
						var nameValues []string
						if nameResult.IsArray() {
							for _, elem := range nameResult.Array() {
								nameValues = append(nameValues, strings.TrimSpace(elem.String()))
							}
						} else {
							nameValues = append(nameValues, strings.TrimSpace(nameResult.String()))
						}

						// Check each extracted name
						for _, extractedName := range nameValues {
							nameMatches := (extractedName == obj.GetName())

							// Check namespace if JSONPath was provided for namespace
							namespaceMatches := true
							if selector.JSONPath.Namespace != "" && namespaceResult.Exists() {
								var namespaceValues []string
								if namespaceResult.IsArray() {
									for _, elem := range namespaceResult.Array() {
										namespaceValues = append(namespaceValues, strings.TrimSpace(elem.String()))
									}
								} else {
									namespaceValues = append(namespaceValues, strings.TrimSpace(namespaceResult.String()))
								}

								// For namespaced objects, check if extracted namespace matches
								namespaceMatches = false
								objNamespace := obj.GetNamespace()
								for _, extractedNamespace := range namespaceValues {
									if extractedNamespace == objNamespace {
										namespaceMatches = true
										break
									}
								}
							}

							// If both name and namespace match, we found a reference
							if nameMatches && namespaceMatches {
								referenceMatches = true
								logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "reference", "reference", selector)
								break
							}
						}

						if referenceMatches {
							break
						}
					}
				}
			}
		}
	}

	return labelSelectorMatches || namedResourceMatches || referenceMatches
}

// IsClaimedWithReference returns true if the given object is claimed by the given permission claim.
// It handles both consumer side (no namespace remapping) and provider side (namespace remapping via APIServiceNamespace).
// The function fetches potentially referenced resources if a reference selector is present.
func IsClaimedWithReference(
	logger klog.Logger,
	obj *unstructured.Unstructured,
	consumerSide bool,
	claim kubebindv1alpha2.PermissionClaim,
	apiServiceExport *kubebindv1alpha2.APIServiceExport,
	providerNamespace string,
	consumerClient dynamicclient.Interface,
	serviceNamespaceLister ServiceNamespaceLister,
) bool {
	logger = logger.V(4).WithValues("gvr", obj.GroupVersionKind().String(), "namespace", obj.GetNamespace(), "name", obj.GetName())
	logger.Info("checking if object is claimed")
	copy := obj.DeepCopy()

	potentiallyReferencedResources := &unstructured.UnstructuredList{}
	var versions []string

	// Handle reference selectors by fetching potentially referenced resources
	if len(claim.Selector.References) != 0 {
		for _, ref := range claim.Selector.References {
			logger := logger.WithValues("referenceGroup", ref.Group, "referenceResource", ref.Resource, "referenceJSONPathName", ref.JSONPath.Name, "referenceJSONPathNamespace", ref.JSONPath.Namespace)
			logger.Info("processing reference selector")
			if len(ref.Versions) > 0 {
				versions = ref.Versions
			} else {
				// Find versions from apiServiceExport resources
				for v := range apiServiceExport.Spec.Resources {
					if ref.Group == apiServiceExport.Spec.Resources[v].Group && ref.Resource == apiServiceExport.Spec.Resources[v].Resource {
						versions = apiServiceExport.Spec.Resources[v].Versions
						break
					}
				}
			}

			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
			defer cancel()

			for _, version := range versions {
				objs, err := consumerClient.Resource(schema.GroupVersionResource{
					Group:    ref.Group,
					Resource: ref.Resource,
					Version:  version,
				}).List(ctx, metav1.ListOptions{})
				if err != nil {
					logger.Error(err, "failed to list objects for reference claim. Invalidating all claim.", "group", ref.Group, "resource", ref.Resource, "version", version)
					return false
				}
				potentiallyReferencedResources.Items = append(potentiallyReferencedResources.Items, objs.Items...)
			}
		}
	}

	logger.Info("checking claim", "consumerSide", consumerSide, "potentiallyReferencedResourcesCount", len(potentiallyReferencedResources.Items))

	if consumerSide {
		logger.Info("checking claim on consumer side")
		result := IsClaimed(logger, claim.Selector, copy, potentiallyReferencedResources)
		logger.Info("IsClaimed result", "result", result)
		return result
	}

	// For provider side, remap the namespace using APIServiceNamespace
	sns, err := serviceNamespaceLister.List(labels.NewSelector())
	if err != nil && !errors.IsNotFound(err) {
		runtime.HandleError(fmt.Errorf("failed to get APIServiceNamespace for claimed check: %w", err))
		return false
	}
	var sn *kubebindv1alpha2.APIServiceNamespace
	for _, candidate := range sns {
		if candidate.Status.Namespace == "" {
			continue // not ready yet
		}
		if obj.GetNamespace() == candidate.Status.Namespace {
			sn = candidate
			break
		}
	}
	if sn != nil {
		copy.SetNamespace(sn.Name)
	}

	result := IsClaimed(logger, claim.Selector, copy, potentiallyReferencedResources)
	logger.Info("IsClaimed result (provider side)", "result", result)
	return result
}

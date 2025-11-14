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

func matchesLabelSelector(logger klog.Logger, selector *metav1.LabelSelector, obj *unstructured.Unstructured) bool {
	if selector == nil {
		return false
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid label selector in permission claim: %w", err))
		return false
	}

	objLabels := obj.GetLabels()
	if objLabels == nil {
		objLabels = make(map[string]string)
	}

	matches := labelSelector.Matches(labels.Set(objLabels))
	if matches {
		logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "labelSelector", "selector", labelSelector.String(), "objectLabels", objLabels)
	}
	return matches
}

func matchesNamedResources(logger klog.Logger, namedResources []kubebindv1alpha2.NamedResource, obj *unstructured.Unstructured) bool {
	if len(namedResources) == 0 {
		return false
	}

	for _, nr := range namedResources {
		if nr.Namespace != "" && nr.Namespace != obj.GetNamespace() {
			continue
		}
		if nr.Name == obj.GetName() {
			logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "namedResource", "namedResource", nr)
			return true
		}
	}
	return false
}

func matchesReferences(logger klog.Logger, references []kubebindv1alpha2.SelectorReference, obj *unstructured.Unstructured, potentiallyReferencedResources *unstructured.UnstructuredList) bool {
	if len(references) == 0 {
		return false
	}

	if potentiallyReferencedResources == nil {
		return false
	}

	for _, refObj := range potentiallyReferencedResources.Items {
		jsonData, err := refObj.MarshalJSON()
		if err != nil {
			continue
		}

		for _, ref := range references {
			selector := ref
			nameResult := gjson.Get(string(jsonData), selector.JSONPath.Name)
			var namespaceResult gjson.Result
			if selector.JSONPath.Namespace != "" {
				namespaceResult = gjson.Get(string(jsonData), selector.JSONPath.Namespace)
			}

			if nameResult.Exists() {
				var nameValues []string
				if nameResult.IsArray() {
					for _, elem := range nameResult.Array() {
						nameValues = append(nameValues, strings.TrimSpace(elem.String()))
					}
				} else {
					nameValues = append(nameValues, strings.TrimSpace(nameResult.String()))
				}

				for _, extractedName := range nameValues {
					nameMatches := (extractedName == obj.GetName())

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

						namespaceMatches = false
						objNamespace := obj.GetNamespace()
						for _, extractedNamespace := range namespaceValues {
							if extractedNamespace == objNamespace {
								namespaceMatches = true
								break
							}
						}
					}

					if nameMatches && namespaceMatches {
						logger.Info("resource matched", "name", obj.GetName(), "namespace", obj.GetNamespace(), "claim-type", "reference", "reference", selector)
						return true
					}
				}
			}
		}
	}
	return false
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

	labelSelectorMatches := matchesLabelSelector(logger, selector.LabelSelector, obj)
	namedResourceMatches := matchesNamedResources(logger, selector.NamedResources, obj)
	referenceMatches := matchesReferences(logger, selector.References, obj, potentiallyReferencedResources)

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

			// TODO(mjudeikis): We should wire informers per GVR from above. Now this is bit hack
			// and there is no context available here.
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

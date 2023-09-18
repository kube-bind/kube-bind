/*
Copyright 2023 The Kube Bind Authors.

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

package clusterscoped

import (
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ClusterNsAnnotationKey is the annotation key to identify the cluster namespace that corresponds to
// provider side copy of a cluster-scoped object. Its value is the corresponding cluster namespace name.
const ClusterNsAnnotationKey = "kube-bind.io/cluster-namespace"

// Prepend adds clusterNs and a dash before name.
func Prepend(name, clusterNs string) string {
	return clusterNs + "-" + name
}

// Behead tries to remove clusterNs and a dash that goes before name.
// If name doesn't start with clusterNs, name is returned unchanged.
func Behead(name, clusterNs string) string {
	return strings.TrimPrefix(name, clusterNs+"-")
}

// SwitchToUpstreamName switches the name of a downstream
// cluster-scoped object by prepending the cluster namespace name.
func SwitchToUpstreamName(obj *unstructured.Unstructured, clusterNs string) {
	downstreamName := obj.GetName()
	upstreamName := Prepend(downstreamName, clusterNs)
	obj.SetName(upstreamName)
}

// SwitchToDownstreamName switches the name of a upstream
// cluster-scoped object by removing the cluster namespace name as the prefix.
func SwitchToDownstreamName(obj *unstructured.Unstructured, clusterNs string) {
	upstreamName := obj.GetName()
	downstreamName := Behead(upstreamName, clusterNs)
	obj.SetName(downstreamName)
}

// InjectClusterNs injects the given cluster namespace (1) as an annotation,
// and (2) as a owner reference to the cluster namespace.
func InjectClusterNs(obj *unstructured.Unstructured, clusterNs, clusterNsUID string) error {
	ans := obj.GetAnnotations()
	existing, foundAn := ans[ClusterNsAnnotationKey]
	if foundAn && existing != clusterNs {
		return errors.New("mismatch between existing cluster namespace and given cluster namespace")
	}

	ors := obj.GetOwnerReferences()
	idx, foundOr := findOwnerReferenceToClusterNs(ors, clusterNs)
	if foundOr && ors[idx].Name != clusterNs {
		return errors.New("mismatch between existing cluster namespace and given cluster namespace")
	}

	if !foundAn {
		if ans == nil {
			ans = map[string]string{}
		}
		ans[ClusterNsAnnotationKey] = clusterNs
		obj.SetAnnotations(ans)
	}

	if !foundOr {
		ors = append(ors, metav1.OwnerReference{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       clusterNs,
			UID:        types.UID(clusterNsUID),
		})
		obj.SetOwnerReferences(ors)
	}

	return nil
}

// ExtractClusterNs extracts the corresponding cluster namespace name
// from a cluster-scoped object by reading the annotation.
func ExtractClusterNs(obj *unstructured.Unstructured) (string, error) {
	ans := obj.GetAnnotations()
	clusterNs, ok := ans[ClusterNsAnnotationKey]
	if !ok {
		return "", errors.New("cluster namespace annotation not found")
	}
	return clusterNs, nil
}

// ClearClusterNs clears the given cluster namespace in a cluster-scoped object,
// including both the annotation and the owner reference.
func ClearClusterNs(obj *unstructured.Unstructured, clusterNs string) error {
	ans := obj.GetAnnotations()
	delete(ans, ClusterNsAnnotationKey)
	obj.SetAnnotations(ans)

	ors := obj.GetOwnerReferences()
	idx, foundOr := findOwnerReferenceToClusterNs(ors, clusterNs)
	if foundOr {
		ors[idx] = ors[len(ors)-1]
		ors = ors[:len(ors)-1]
		obj.SetOwnerReferences(ors)
	}

	return nil
}

// TranslateFromDownstream mutates a cluster-scoped object in place by injecting the cluster namespace
// and switching to its corresponding upstream name
func TranslateFromDownstream(obj *unstructured.Unstructured, clusterNs, clusterNsUID string) error {
	copy := obj.DeepCopy()
	err := InjectClusterNs(copy, clusterNs, clusterNsUID)
	if err != nil {
		return err
	}
	SwitchToUpstreamName(copy, clusterNs)
	*obj = *copy
	return nil
}

// TranslateFromUpstream mutates a cluster-scoped object in place by clearing the injected cluster namespace
// and switching to its corresponding downstream name
func TranslateFromUpstream(obj *unstructured.Unstructured) error {
	clusterNs, err := ExtractClusterNs(obj)
	if err != nil {
		return err
	}

	copy := obj.DeepCopy()
	err = ClearClusterNs(copy, clusterNs)
	if err != nil {
		return err
	}
	SwitchToDownstreamName(copy, clusterNs)
	*obj = *copy
	return nil
}

func findOwnerReferenceToClusterNs(ors []metav1.OwnerReference, clusterNs string) (int, bool) {
	if ors == nil {
		return -1, false
	}
	for i, or := range ors {
		if or.Kind == "Namespace" && or.Name == clusterNs {
			return i, true
		}
	}
	return -1, false
}

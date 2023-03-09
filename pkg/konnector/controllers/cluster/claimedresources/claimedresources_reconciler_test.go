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

package claimedresources

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

func TestDownstreamCreation(t *testing.T) {
	t.Parallel()

	var createdObj *unstructured.Unstructured
	r := readReconciler{
		getServiceNamespace: defaultNamespace,
		getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "dummy",
						"namespace": "cluster-x-default",
					},
				},
			}, nil
		},
		getConsumerObject: notFound,
		createConsumerObject: func(ctx context.Context, ob *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			createdObj = ob
			return ob, nil
		},
	}

	err := r.reconcile(context.TODO(), "cluster-x-default", "dummy")
	if err != nil {
		t.Fatal(err)
	}

	if createdObj == nil {
		t.Error("reconcile did not create an object", createdObj)
	}

	if v, ok := createdObj.GetAnnotations()["kube-bind.io/claimedresource"]; !ok || v != "true" {
		t.Error("created object did not have 'kube-bind.io/claimedresource: true' annotation")
	}
}

func defaultNamespace(upstreamNamespace string) (*v1alpha1.APIServiceNamespace, error) {
	return &v1alpha1.APIServiceNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "kube-bind",
		},
		Spec: v1alpha1.APIServiceNamespaceSpec{},
		Status: v1alpha1.APIServiceNamespaceStatus{
			Namespace: "cluster-x-default",
		},
	}, nil
}

func notFound(_ context.Context, ns, name string) (*unstructured.Unstructured, error) {
	return nil, errors.NewNotFound(v1.Resource("Secret"), name)
}

func TestDownstreamDeletion(t *testing.T) {
	t.Parallel()

	var deleteNsn struct {
		ns, name string
	}
	r := readReconciler{
		getServiceNamespace: defaultNamespace,
		getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
			return nil, errors.NewNotFound(v1.Resource("Secret"), name)
		},
		getConsumerObject: notFound,
		deleteConsumerObject: func(ctx context.Context, ns, name string) error {
			deleteNsn = struct {
				ns   string
				name string
			}{ns: ns, name: name}
			return nil
		},
	}

	err := r.reconcile(context.TODO(), "cluster-x-default", "dummy")
	if err != nil {
		t.Fatal(err)
	}

	if deleteNsn.name != "dummy" || deleteNsn.ns != "default" {
		t.Error("reconcile deleted the wrong object", deleteNsn)
	}
}

func TestDownstreamDeletionAlreadyGone(t *testing.T) {
	t.Parallel()

	var deleteNsn struct {
		ns, name string
	}
	r := readReconciler{
		getServiceNamespace: defaultNamespace,
		getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
			return nil, errors.NewNotFound(v1.Resource("Secret"), name)
		},
		getConsumerObject: notFound,
		deleteConsumerObject: func(ctx context.Context, ns, name string) error {
			deleteNsn = struct {
				ns   string
				name string
			}{ns: ns, name: name}
			return errors.NewNotFound(v1.Resource("Secret"), name)

		},
	}

	err := r.reconcile(context.TODO(), "cluster-x-default", "dummy")
	if err != nil {
		t.Fatal(err)
	}

	if deleteNsn.name != "dummy" || deleteNsn.ns != "default" {
		t.Error("reconcile deleted the wrong object", deleteNsn)
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	var updateObj *unstructured.Unstructured
	r := readReconciler{
		getServiceNamespace: defaultNamespace,
		getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "dummy",
						"namespace": "cluster-x-default",
					},
					"data": map[string]interface{}{
						"username": "user",
						"password": "pass",
					},
				},
			)
			return obj, nil
		},
		getConsumerObject: func(ctx context.Context, ns, name string) (*unstructured.Unstructured, error) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "dummy",
						"namespace": "default",
						"annotations": map[string]interface{}{
							"kube-bind.io/claimedresource": "true",
						},
					},
					"data": map[string]interface{}{
						"username": "user",
					},
				},
			)
			return obj, nil
		},
		updateConsumerObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			updateObj = obj
			return obj, nil
		},
	}

	err := r.reconcile(context.TODO(), "cluster-x-default", "dummy")
	if err != nil {
		t.Fatal(err)
	}

	if updateObj == nil {
		t.Fatal("update object nil")
	}
	if v, ok := updateObj.GetAnnotations()["kube-bind.io/claimedresource"]; !ok || v != "true" {
		t.Error("updated object did not have 'kube-bind.io/claimedresource: true' annotation")
	}
}
func TestUpdateNotNeeded(t *testing.T) {
	t.Parallel()

	r := readReconciler{
		getServiceNamespace: defaultNamespace,
		getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "dummy",
						"namespace": "cluster-x-default",
					},
					"data": map[string]interface{}{
						"username": "user",
						"password": "pass",
					},
				},
			)
			return obj, nil
		},
		getConsumerObject: func(ctx context.Context, ns, name string) (*unstructured.Unstructured, error) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "dummy",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"username": "user",
						"password": "pass",
					},
				},
			)
			obj.SetAnnotations(map[string]string{
				"kube-bind.io/claimedresource": "true",
			})
			return obj, nil
		},
		updateConsumerObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			t.Fatal("update function called although not needed", obj)
			return nil, nil
		},
	}

	err := r.reconcile(context.TODO(), "cluster-x-default", "dummy")
	if err != nil {
		t.Fatal(err)
	}
}

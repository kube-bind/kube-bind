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

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/martinlindhe/base36"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IdentityAnnotationKey       = "backend.kube-bind.io/identity"
	AuthorAnnotationKey         = "backend.kube-bind.io/author"
	legacyIdentityAnnotationKey = "example-backend.kube-bind.io/identity"
)

func handleLegacyAnnotations(ctx context.Context, cl client.Client, namespace *corev1.Namespace, id string) error {
	if namespace.Annotations == nil {
		return nil
	}

	legacyValue, hasLegacy := namespace.Annotations[legacyIdentityAnnotationKey]
	currentValue, hasCurrent := namespace.Annotations[IdentityAnnotationKey]

	if hasLegacy && (!hasCurrent || currentValue != id) && legacyValue == id {
		original := namespace.DeepCopy()
		if namespace.Annotations == nil {
			namespace.Annotations = map[string]string{}
		}
		namespace.Annotations[IdentityAnnotationKey] = id
		delete(namespace.Annotations, legacyIdentityAnnotationKey)
		return cl.Patch(ctx, namespace, client.MergeFrom(original))
	}

	return nil
}

func CreateNamespace(ctx context.Context, client client.Client, generateNamePrefix, identity, author string) (*corev1.Namespace, error) {
	name := identityHash(identity)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", generateNamePrefix, name),
			Annotations: map[string]string{
				IdentityAnnotationKey: identity,
				AuthorAnnotationKey:   author,
			},
		},
	}

	err := client.Create(ctx, namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	} else if errors.IsAlreadyExists(err) {
		err := client.Get(ctx, types.NamespacedName{Name: namespace.Name}, namespace)
		if err != nil {
			return nil, err
		}

		if namespace.Annotations[IdentityAnnotationKey] != identity && namespace.Annotations[legacyIdentityAnnotationKey] != identity {
			return nil, errors.NewAlreadyExists(corev1.Resource("namespace"), namespace.Name)
		}

		if err := handleLegacyAnnotations(ctx, client, namespace, identity); err != nil {
			return nil, err
		}
	}

	return namespace, nil
}

func identityHash(userName string) string {
	hash := sha256.Sum224([]byte(userName))
	return strings.ToLower(base36.EncodeBytes(hash[:8]))
}

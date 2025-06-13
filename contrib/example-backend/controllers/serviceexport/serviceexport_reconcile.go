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

package serviceexport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	kubebindhelpers "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	getCRD               func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getAPIResourceSchema func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error)
	deleteServiceExport  func(ctx context.Context, namespace, name string) error

	requeue func(export *kubebindv1alpha2.APIServiceExport)
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) error {
	var errs []error

	if specChanged, err := r.ensureSchema(ctx, export); err != nil {
		errs = append(errs, err)
	} else if specChanged {
		// TODO: This should be separate controller for apiresourceschemas.
		// This is wrong place now.
		//	r.requeue(export)
		return nil
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureSchema(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) (specChanged bool, err error) {
	logger := klog.FromContext(ctx)

	var leafHashes []string
	for _, resourceRef := range export.Spec.Resources {
		if resourceRef.Type != "APIResourceSchema" {
			logger.V(1).Info("Skipping unsupported resource type", "type", resourceRef.Type)
			continue
		}

		schema, err := r.getAPIResourceSchema(ctx, resourceRef.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}

		hash := kubebindhelpers.APIResourceSchemaCRDSpecHash(&schema.Spec.APIResourceSchemaCRDSpec)
		leafHashes = append(leafHashes, hash)
	}

	hashOfHashes := hashOfHashes(leafHashes)

	if export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] != hashOfHashes {
		// both exist, update APIServiceExport
		logger.V(1).Info("Updating APIServiceExport. Hash mismatch", "hash", hashOfHashes, "expected", export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey])
		if export.Annotations == nil {
			export.Annotations = map[string]string{}
		}
		export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] = hashOfHashes
		return true, nil
	}

	conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionProviderInSync)

	return false, nil
}

func hashOfHashes(hashes []string) string {
	hexHashes := append([]string{}, hashes...)

	sort.Strings(hexHashes)

	rootHasher := sha256.New()
	for _, h := range hexHashes {
		rootHasher.Write([]byte(h))
	}
	return hex.EncodeToString(rootHasher.Sum(nil))
}

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

package serviceexportresource

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type reconciler struct {
	listServiceExports          func(ns, group, resource string) ([]*kubebindv1alpha1.ServiceExport, error)
	deleteServiceExportResource func(ctx context.Context, ns, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha1.ServiceExportResource) error {
	logger := klog.FromContext(ctx)

	exports, err := r.listServiceExports(export.Namespace, export.Spec.Group, export.Spec.Names.Plural)
	if err != nil {
		return err
	}
	if len(exports) == 0 {
		// No ServiceExports => delete SER
		logger.V(1).Info("Deleting ServiceExportResource because not ServiceExport needs it")
		if err := r.deleteServiceExportResource(ctx, export.Namespace, export.Name); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

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

package admission

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	"github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
)

// APIServiceExportRequestValidator validates APIServiceExportRequest objects.
type APIServiceExportRequestValidator struct {
	decoder                admission.Decoder
	manager                mcmanager.Manager
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	schemaSource           string
}

// NewAPIServiceExportRequestValidator creates a new validator for APIServiceExportRequest.
func NewAPIServiceExportRequestValidator(
	manager mcmanager.Manager,
	decoder admission.Decoder,
	scope kubebindv1alpha2.InformerScope,
	isolation kubebindv1alpha2.Isolation,
	schemaSource string,
) *APIServiceExportRequestValidator {
	return &APIServiceExportRequestValidator{
		decoder:                decoder,
		manager:                manager,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		schemaSource:           schemaSource,
	}
}

func (v *APIServiceExportRequestValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx)
	ctx = klog.NewContext(ctx, logger)

	obj := &kubebindv1alpha2.APIServiceExportRequest{}
	if err := v.decoder.Decode(req, obj); err != nil {
		logger.Error(err, "Admission webhook: failed to decode APIServiceExportRequest")
		return admission.Errored(http.StatusBadRequest, err)
	}

	logger.Info("Admission webhook: decoded request", "resources", len(obj.Spec.Resources), "informerScope", v.informerScope)

	clusterName := ""
	cl, err := v.manager.GetCluster(ctx, clusterName)
	if err != nil {
		clusterName = "default"
		cl, err = v.manager.GetCluster(ctx, clusterName)
		if err != nil {
			logger.Info("Admission webhook: failed to get cluster for validation", "error", err)
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to get cluster: %w", err))
		}
	}
	client := cl.GetClient()

	if err := v.validateAPIServiceExportRequest(ctx, client, obj); err != nil {
		return admission.Denied(err.Error())
	}

	logger.Info("Admission webhook: validation allowed")
	return admission.Allowed("")
}

func (v *APIServiceExportRequestValidator) validateAPIServiceExportRequest(ctx context.Context, cl client.Client, req *kubebindv1alpha2.APIServiceExportRequest) error {
	logger := klog.FromContext(ctx)
	logger.Info("Admission webhook: validating APIServiceExportRequest", "resources", len(req.Spec.Resources), "permissionClaims", len(req.Spec.PermissionClaims))

	exportedSchemas, err := v.getExportedSchemas(ctx, cl)
	if err != nil {
		return err
	}

	if len(exportedSchemas) == 0 {
		return fmt.Errorf("no exported schemas found")
	}

	first := apiextensionsv1.ResourceScope("")
	for _, res := range req.Spec.Resources {
		boundSchema, ok := exportedSchemas[res.ResourceGroupName()]
		if !ok {
			return fmt.Errorf("schema %s not found", res.ResourceGroupName())
		}

		if boundSchema.Spec.Scope == apiextensionsv1.ClusterScoped && v.informerScope != kubebindv1alpha2.ClusterScope {
			return fmt.Errorf("resource %s/%s has scope %q which is incompatible with backend informer scope %q", res.Group, res.Resource, boundSchema.Spec.Scope, v.informerScope)
		}

		if first == apiextensionsv1.ResourceScope("") {
			first = boundSchema.Spec.Scope
			continue
		}
		if boundSchema.Spec.Scope != first {
			return fmt.Errorf("different scopes found for claimed resources: %v", boundSchema.Name)
		}
	}

	for _, claim := range req.Spec.PermissionClaims {
		if !isClaimableAPI(claim) {
			return fmt.Errorf("resource %s is not a valid claimable API", claim.GroupResource.String())
		}
	}

	seenGroupResources := make(map[string]bool)
	for _, claim := range req.Spec.PermissionClaims {
		key := claim.Group + "/" + claim.Resource
		if seenGroupResources[key] {
			return fmt.Errorf("duplicate permission claim found for group/resource %s", claim.GroupResource.String())
		}
		seenGroupResources[key] = true
	}

	return nil
}

func (v *APIServiceExportRequestValidator) getExportedSchemas(ctx context.Context, cl client.Client) (kubebindv1alpha2.ExportedSchemas, error) {
	parts := strings.SplitN(v.schemaSource, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed schema source: %q", v.schemaSource)
	}

	gvk := schema.GroupVersionKind{
		Kind:    parts[0],
		Version: parts[1],
		Group:   parts[2],
	}

	// Ensure we have the List kind
	listGVK := gvk
	if !strings.HasSuffix(listGVK.Kind, "List") {
		listGVK.Kind += "List"
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(listGVK)

	labelSelector := labels.Set{
		resources.ExportedCRDsLabel: "true",
	}

	listOpts := []client.ListOption{}
	listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: labelSelector.AsSelector()})

	if err := cl.List(ctx, list, listOpts...); err != nil {
		return nil, err
	}

	boundSchemas := make(kubebindv1alpha2.ExportedSchemas, len(list.Items))
	for _, item := range list.Items {
		boundSchema, err := helpers.UnstructuredToBoundSchema(item)
		if err != nil {
			return nil, err
		}
		boundSchemas[boundSchema.ResourceGroupName()] = boundSchema
	}

	return boundSchemas, nil
}

func isClaimableAPI(claim kubebindv1alpha2.PermissionClaim) bool {
	for _, api := range kubebindv1alpha2.ClaimableAPIs {
		if claim.Group == api.GroupVersionResource.Group && claim.Resource == api.Names.Plural {
			return true
		}
	}
	return false
}

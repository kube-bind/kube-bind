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

package v1alpha1

// PermissionClaim specifies permissions for a service provider to access
// resources and fields in a consumer cluster. A PermissionClaim must be
// accepted before the specified permissions are effective. Permission claims
// are implemented through the konnector by syncing the right objects and
// fields in the right direction.
//
// Permission claims distinguish objects owned by the provider and objects
// owned by the consumer. The owner of an object determines which side is the
// source of truth for the object, i.e. whether the object is synced from the
// consumer to the provider cluster or vice versa. Exceptions can be specified
// for individual JSON Paths to be owned by the other side. Metadata in general
// is not synced. Exceptions for labels and annotations can be specified.
//
// Ownership of an object is determined by the `kube-bind.io/owner` annotation.
// Objects on the consumer cluster are owned by the consumer by default. Objects
// on the provider cluster are owned by the provider by default. The annotation
// is only set if the object is owned by the respective other side.
type PermissionClaim struct {
	GroupResource `json:","`

	// version is the version of the claimed resource.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Version string `json:"version"`

	// selector restricts which objects of the given resource are being claimed.
	// If unset, all objects across all namespaces are being claimed, both
	// consumer- and provider owned.
	//
	// +optional
	// +kubebuilder:default:={}
	ObjectSelector *ObjectSelector `json:"objectSelector,omitempty"`

	// read claims read access for the provider to matching objects, excluding
	// labels and annotations by default. Read access is realized by syncing
	// the objects or fields from the consumer cluster to the provider cluster.
	//
	// +optional
	// +kubebuilder:default={}
	Read *PermissionClaimReadOptions `json:"read,omitempty"`

	// create determines whether the provider can create new objects in the
	// consumer cluster by syncing a provider-owned source to the consumer
	// cluster. Created objects on the consumer cluster are marked as owned by
	// the provider by default.
	//
	// Note that create permissions do not imply update permissions.
	//
	// +optional
	Create *PermissionClaimCreateOptions `json:"create,omitempty"`

	// onConflict determines how conflicts between objects on the consumer
	// and provider clusters will be resolved.
	//
	// +optional
	// +kubebuilder:default:={}
	OnConflict *PermissionClaimOnConflictOptions `json:"onConflict,omitempty"`

	// update lists which updates to objects on the consumer cluster are claimed.
	// By default, the whole object is synced, but metadata is not.
	//
	// Note that update permissions do not imply create permissions.
	//
	// +optional
	Update *PermissionClaimUpdateOptions `json:"update,omitempty"`

	// ownerTransfer determines how ownership of objects is transferred between
	// the consumer and the provider. By default, no transfer happens. If set to
	// Donate, objects created by the provider on the consumer cluster are
	// immediately donated to the consumer. If set to Adopt, objects created by
	// the consumer on the consumer cluster are immediately owned by the
	// provider and synced to the provider cluster. Ownership determines the
	// direction of synchronization.
	//
	// +kubebuilder:validation:Enum=Donate;Adopt;""
	// +optional
	OwnerTransfer OwnerTransfer `json:"ownerTransfer,omitempty"`
}

type OwnerTransfer string

const (
	OwnerTransferNone   OwnerTransfer = ""
	OwnerTransferDonate OwnerTransfer = "Donate"
	OwnerTransferAdopt  OwnerTransfer = "Adopt"
)

type PermissionClaimReadOptions struct {
	// labels is a list of label key wildcard patterns that are synchronized
	// from the consumer to the provider on consumer-owned objects.
	//
	// +optional
	Labels []Matcher `json:"labels,omitempty"`

	// LabelsOnProviderOwnedObjects is a list of claimed label key wildcard
	// patterns that are synchronized from the consumer cluster to the provider
	// on objects owned by the provider.
	//
	// +optional
	LabelsOnProviderOwnedObjects []Matcher `json:"labelsOnProviderOwnedObjects,omitempty"`

	// annotations is a list of claimed annotation key wildcard patterns
	// that are synchronized from the consumer cluster to the provider on
	// objects that are owned by the consumer.
	//
	// +optional
	Annotations []Matcher `json:"annotations,omitempty"`

	// overrideAnnotations is a list of claimed annotation key wildcard
	// patterns that are synchronized from the consumer cluster to the provider
	// on provider-owned objects.
	//
	// +optional
	AnnotationsOnProviderOwnedObjects []Matcher `json:"annotationsOnProviderOwnedObjects,omitempty"`
}

type Matcher struct {
	// pattern is a wildcard pattern that is matched against the key. This means
	// it is either a literal string or starts or ends in `*` but is not '*'
	// itself.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Pattern string `json:"pattern,omitempty"`
}

type PermissionClaimOnConflictOptions struct {
	// recreateWhenConsumerSideDeleted set to true (the default) means the provider will recreate the object
	// in case the object is missing on the consumer cluster, but has been synchronized before.
	//
	// If set to false, deleted provider-owned objects get deleted on the provider cluster as well.
	//
	// +kubebuilder:default:=true
	RecreateWhenConsumerSideDeleted bool `json:"recreateWhenConsumerSideDeleted"`
}

type PermissionClaimCreateOptions struct {
	// replaceExisting means that an existing object owned by the consumer will
	// be replaced by the provider object.
	//
	// If not true, and a conflicting consumer object exists, it is not touched.
	//
	// +optional
	ReplaceExisting bool `json:"replaceExisting,omitempty"`
}

type PermissionClaimUpdateOptions struct {
	// fields are a list of JSON Paths in consumer-owned objects on the consumer
	// cluster that the provider wants to control.
	//
	// This is ignored for provider-owned objects.
	//
	// +optional
	Fields []string `json:"fields,omitempty"`

	// preserving is a list of JSON Paths in provider-owned objects on the
	// consumer cluster that the consumer keeps controlling, i.e. that are not
	// overwritten by the provider, but synced back to the provider side.
	//
	// This is ignored for consumer-owned objects.
	//
	// +optional
	Preserving []string `json:"preserving,omitempty"`

	// alwaysRecreate set to true means that matching objects will be deleted
	// and recreated on update. This is useful for immutable objects.
	//
	// This does not apply to metadata field updates.
	//
	// +optional
	AlwaysRecreate bool `json:"alwaysRecreate,omitempty"`

	// labels is a list of label key wildcard patterns that are synced from the
	// provider to the consumer for provider-owned objects.
	//
	// By default, no labels are synced.
	//
	// +optional
	Labels []Matcher `json:"labels,omitempty"`

	// labelsOnConsumerOwnedObjects is a list of label key wildcard patterns
	// that are synced from the provider to the consumer for consumer-owner
	// objects.
	//
	// By default, no labels are synced.
	//
	// +optional
	LabelsOnConsumerOwnedObjects []Matcher `json:"labelsOnConsumerOwnedObjects,omitempty"`

	// annotations is a list of annotation key wildcard patterns that are synced
	// from the provider to the consumer for provider-owned objects.
	//
	// By default, no annotations are synced.
	//
	// +optional
	Annotations []Matcher `json:"annotations,omitempty"`

	// annotationsOnConsumerOwnedObjects is a list of annotation key wildcard
	// patterns that are synchronized from the provider to the consumer for
	// consumer-owned objects.
	//
	// By default, no annotations are synced.
	//
	// +optional
	AnnotationsOnConsumerOwnedObjects []Matcher `json:"annotationsOnConsumerOwnedObjects,omitempty"`
}

type ObjectSelector struct {
	// names is a list of values selecting by metadata.name, or "*" which
	// matches all names.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(n, n.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?|[*]$'))",message="only names or * are allowed"
	// +kubebuilder:default:={"*"}
	// +optional
	Names []string `json:"names,omitempty"`

	// namespaces is a list of values selecting by metadata.namespace, or "*"
	// which matches all namespaces, or empty string that matches cluster-scoped
	// resources.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(n, n.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?|[*]?$'))",message="only namespace names,* or empty string are allowed"
	// +kubebuilder:default:={"*"}
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// labelSelectors is a disjunctive list of label selectors, following the
	// same rules as kubernetes label selectors, see https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/.
	LabelSelectors []map[string]string `json:"labelSelectors,omitempty"`

	// fieldSelectors is a disjunctive list of field selectors, following the
	// same rules as kubernetes field selectors, see https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/,
	// i.e. each field selector can be a conjunction of requirements.
	FieldSelectors []string `json:"fieldSelectors,omitempty"`

	// owner set means that resources of a specific owner are selected and those
	// owned by the other side are ignored. A resource on the consumer cluster
	// is owned by the consumer by default, if not marked as owned by the
	// provider through the `kube-bind.io/owner=provider` annotation.
	//
	// +kubebuilder:validation:Enum=Provider;Consumer
	// +optional
	Owner PermissionClaimResourceOwner `json:"owner,omitempty"`
}

type PermissionClaimResourceOwner string

const (
	// Provider means that the owner of the resource is the Provider.
	Provider PermissionClaimResourceOwner = "Provider"

	// Consumer means that the owner of the resource is the Consumer.
	Consumer PermissionClaimResourceOwner = "Consumer"
)

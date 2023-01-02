/*
Copyright AppsCode Inc. and Contributors

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

import (
	"strings"

	kmapi "kmodules.xyz/client-go/api/v1"
	"kmodules.xyz/client-go/apiextensions"
	"kmodules.xyz/resource-metadata/crds"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func (v ResourceDescriptor) CustomResourceDefinition() *apiextensions.CustomResourceDefinition {
	return crds.MustCustomResourceDefinition(SchemeGroupVersion.WithResource(ResourceResourceDescriptors))
}

func (v ResourceDescriptor) IsValid() error {
	return nil
}

// MarshalYAML implements https://pkg.go.dev/gopkg.in/yaml.v2#Marshaler
func (rd ResourceDescriptor) ToYAML() ([]byte, error) {
	if rd.Spec.Validation != nil &&
		rd.Spec.Validation.OpenAPIV3Schema != nil {

		var mc crdv1.JSONSchemaProps
		err := yaml.Unmarshal([]byte(ObjectMetaSchema), &mc)
		if err != nil {
			return nil, err
		}
		if rd.Spec.Resource.Scope == kmapi.ClusterScoped {
			delete(mc.Properties, "namespace")
		}
		rd.Spec.Validation.OpenAPIV3Schema.Properties["metadata"] = mc
		delete(rd.Spec.Validation.OpenAPIV3Schema.Properties, "status")
	}

	data, err := yaml.Marshal(rd)
	if err != nil {
		return nil, err
	}

	return FormatMetadata(data)
}

func IsOfficialType(group string) bool {
	switch {
	case group == "":
		return true
	case !strings.ContainsRune(group, '.'):
		return true
	case group == "k8s.io" || strings.HasSuffix(group, ".k8s.io"):
		return true
	case group == "kubernetes.io" || strings.HasSuffix(group, ".kubernetes.io"):
		return true
	case group == "x-k8s.io" || strings.HasSuffix(group, ".x-k8s.io"):
		return true
	default:
		return false
	}
}

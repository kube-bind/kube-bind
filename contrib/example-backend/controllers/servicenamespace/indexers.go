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

package servicenamespace

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	ByServiceNamespace = "byServiceNamespace"

	serviceNamespaceAnnotationKey = "kube-bind.io/service-namespace"
)

func IndexByServiceNamespace(obj interface{}) ([]string, error) {
	a, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	if value, found := a.GetAnnotations()[serviceNamespaceAnnotationKey]; found {
		return []string{value}, nil
	}

	return nil, nil
}

func ServiceNamespaceAnnotationValue(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}

func ServiceNamespaceFromAnnotation(value string) (ns string, name string, err error) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid anotation %s=%q", serviceNamespaceAnnotationKey, value)
	}
	return parts[0], parts[1], nil
}

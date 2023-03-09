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

package exporttemplate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crd "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	templates "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned/fake"
)

var mangodb = apiextensions.CustomResourceDefinition{
	ObjectMeta: v1.ObjectMeta{
		Name: "mangodbs.mangodb.com",
	},
	Spec: apiextensions.CustomResourceDefinitionSpec{
		Group: "mangodb.com",
		Scope: apiextensions.NamespaceScoped,
		Names: apiextensions.CustomResourceDefinitionNames{
			Plural: "mangodbs",
			Kind:   "MangoDB",
		},
	},
}

var dummy = apiextensions.CustomResourceDefinition{
	ObjectMeta: v1.ObjectMeta{
		Name: "dummies.example.com",
	},
	Spec: apiextensions.CustomResourceDefinitionSpec{
		Group: "example.com",
		Scope: apiextensions.NamespaceScoped,
		Names: apiextensions.CustomResourceDefinitionNames{
			Plural: "dummies",
			Kind:   "dummies",
		},
	},
}

var export = v1alpha1.APIServiceExportTemplate{
	Spec: v1alpha1.APIServiceExportTemplateSpec{
		APIServiceSelector: v1alpha1.APIServiceSelector{
			Resource: "mangodbs",
			Group:    "mangodb.com",
		},
	},
	ObjectMeta: v1.ObjectMeta{
		Name:      "mangodb.com",
		Namespace: "cluster-x",
	},
}

func TestListCRDsForAPIServiceExport(t *testing.T) {
	t.Parallel()

	c := crd.NewSimpleClientset(&mangodb, &dummy)
	templatesClient := templates.NewSimpleClientset(&export)

	ix := Index{
		templates: templatesClient,
		crds:      c,
	}

	crdList, err := ix.GetExported(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, []apiextensions.CustomResourceDefinition{mangodb}, crdList)
}

func TestGetAPIServiceExportTemplates(t *testing.T) {
	t.Parallel()

	c := crd.NewSimpleClientset(&mangodb, &dummy)
	templatesClient := templates.NewSimpleClientset(&export)

	ix := Index{
		templates: templatesClient,
		crds:      c,
	}

	exported, err := ix.TemplateFor(context.TODO(), mangodb.Spec.Group, mangodb.Spec.Names.Plural)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, export, exported)
}

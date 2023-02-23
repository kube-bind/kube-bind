package exporttemplate

// TODO by namespace
// TODO cached client
// TODO find a better name or reorganize into different packages
import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crd "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	templates "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
)

type Index struct {
	templates templates.Interface
	crds      crd.Interface
	clusterNs string
}

func NewCatalogue(r *rest.Config) Index {
	crdClient := crd.NewForConfigOrDie(r)

	templateClient := templates.NewForConfigOrDie(r)

	return Index{
		templates: templateClient,
		crds:      crdClient,
	}
}

func (i Index) GetExported(ctx context.Context) ([]apiextensionsv1.CustomResourceDefinition, error) {
	list, err := i.crds.ApiextensionsV1().CustomResourceDefinitions().List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	exports, err := i.templates.KubeBindV1alpha1().APIServiceExportTemplates(i.clusterNs).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	exported := []apiextensionsv1.CustomResourceDefinition{}

	for _, ex := range exports.Items {
		for _, c := range list.Items {
			s := ex.Spec.APIServiceSelector
			if s.Group == c.Spec.Group && s.Resource == c.Spec.Names.Plural {
				exported = append(exported, c)
			}
		}
	}

	if exported == nil {
		return nil, fmt.Errorf("no exported resources")
	}

	return exported, nil
}

func (i Index) TemplateFor(ctx context.Context, group, resource string) (kubebindv1alpha1.APIServiceExportTemplate, error) {
	exports, err := i.templates.KubeBindV1alpha1().APIServiceExportTemplates(i.clusterNs).List(ctx, v1.ListOptions{})
	if err != nil {
		return kubebindv1alpha1.APIServiceExportTemplate{}, nil
	}

	for _, e := range exports.Items {
		if e.Spec.APIServiceSelector.Resource == resource && e.Spec.APIServiceSelector.Group == group {
			return e, nil
		}
	}

	return kubebindv1alpha1.APIServiceExportTemplate{}, fmt.Errorf("not found: %s/%s", group, resource)
}

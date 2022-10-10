package indexers

import (
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

const (
	ServiceExportByServiceExportResource    = "serviceExportByServiceExportResource"
	ServiceExportByCustomResourceDefinition = "serviceExportByCustomResourceDefinition"
)

func IndexServiceExportByServiceExportResource(obj interface{}) ([]string, error) {
	export, ok := obj.(*v1alpha1.ServiceExport)
	if !ok {
		return nil, nil
	}

	grs := []string{}
	for _, gr := range export.Spec.Resources {
		grs = append(grs, export.Namespace+"/"+gr.Resource+"."+gr.Group)
	}
	return grs, nil
}

func IndexServiceExportByCustomResourceDefinition(obj interface{}) ([]string, error) {
	export, ok := obj.(*v1alpha1.ServiceExport)
	if !ok {
		return nil, nil
	}

	grs := []string{}
	for _, gr := range export.Spec.Resources {
		grs = append(grs, gr.Resource+"."+gr.Group)
	}
	return grs, nil
}

package models

import (
	"errors"
	"fmt"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions"
	kubernetesinformers "k8s.io/client-go/informers"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ProviderInfo struct {
	Namespace, ClusterID, ConsumerSecretRefKey string
	Config                                     *rest.Config
	BindClient                                 *bindclient.Clientset
	KubeClient                                 *kubernetesclient.Clientset
	BindInformer                               bindinformers.SharedInformerFactory
	KubeInformer                               kubernetesinformers.SharedInformerFactory
}

func GetProviderInfoWithClusterID(providerInfos []*ProviderInfo, clusterID string) (*ProviderInfo, error) {
	for _, info := range providerInfos {
		if info.ClusterID == clusterID {
			return info, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("no provider information found with cluster id: %s", clusterID))
}

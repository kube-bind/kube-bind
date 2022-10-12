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

package resources

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
)

// EnsureServiceBindingSession create service provider session object after a successful bind authentication.
func EnsureServiceBindingSession(ctx context.Context, clusterName, authSecretName, sessionID, namespace string, client bindclient.Interface) (string, error) {
	serviceBindingSessionName := fmt.Sprintf("%s-service-binding-session", clusterName)
	serviceBindingSession := &kubebindv1alpha1.ServiceBindingSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceBindingSessionName,
			Namespace: namespace,
		},
		Spec: kubebindv1alpha1.ServiceBindingSessionSpec{
			SessionID: sessionID,
			KubeconfigSecretRef: kubebindv1alpha1.LocalSecretKeyRef{
				Name: authSecretName,
				Key:  "kubeconfig",
			},
			AccessTokenRef: kubebindv1alpha1.LocalSecretKeyRef{
				Name: authSecretName,
				Key:  "accessToken",
			},
		},
	}

	_, err := client.KubeBindV1alpha1().ServiceBindingSessions(namespace).Create(ctx, serviceBindingSession, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}

	return serviceBindingSessionName, nil
}

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

package base

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func ParseRemoteKubeconfig(kubeconfig []byte) (host string, ns string, err error) {
	config, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return "", "", err
	}
	if _, found := config.Contexts[config.CurrentContext]; !found {
		return "", "", fmt.Errorf("current context %q of remote kubeconfig not found", config.CurrentContext)
	}
	cluster, found := config.Clusters[config.Contexts[config.CurrentContext].Cluster]
	if !found {
		return "", "", fmt.Errorf("cluster %q in current context %q of remote kubeconfig not found", config.Contexts[config.CurrentContext].Cluster, config.CurrentContext)
	}

	return cluster.Server, config.Contexts[config.CurrentContext].Namespace, nil
}

func FindRemoteKubeconfig(ctx context.Context, kubeClient *kubernetes.Clientset, remoteNamespace string, remoteHost string) (string, error) {
	logger := klog.FromContext(ctx)

	secrets, err := kubeClient.CoreV1().Secrets("kube-bind").List(ctx, v1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, s := range secrets.Items {
		logger := logger.WithValues("namespace", "kube-bind", "name", s.Name)
		bs, found := s.Data["kubeconfig"]
		if !found {
			logger.V(6).Info("secret does not contain kubeconfig")
			continue
		}
		host, ns, err := ParseRemoteKubeconfig(bs)
		if err != nil {
			logger.V(6).Info("failed to parse kubeconfig", "error", err)
			continue
		}
		if ns != remoteNamespace {
			logger.V(6).Info("secret current context namespace not set")
			continue
		}
		if host != remoteHost {
			logger.V(6).Info("secret current context host mismatch")
			continue
		}

		return s.Name, nil
	}
	return "", nil
}

// EnsureKubeconfigSecret creates a secret which contains the service binding authenticated data such as
// the binding session id and the kubeconfig of the service provider cluster. If it is pre-existing, the kubeconfig
// is updated.
//
// It does special checking that only kubeconfigs with the same host and default namespace are updated.
func EnsureKubeconfigSecret(ctx context.Context, kubeconfig, name string, client kubernetes.Interface) (*corev1.Secret, bool, error) {
	remoteHost, remoteNamespace, err := ParseRemoteKubeconfig([]byte(kubeconfig))
	if err != nil {
		return nil, false, err
	}

	if name == "" {
		secret := &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Namespace:    "kube-bind",
				GenerateName: "kubeconfig-",
			},
			Data: map[string][]byte{
				"kubeconfig": []byte(kubeconfig),
			},
		}

		secret, err := client.CoreV1().Secrets("kube-bind").Create(ctx, secret, v1.CreateOptions{})
		if err != nil {
			return nil, false, err
		}
		return secret, true, nil
	}

	// update existing secret
	var secret *corev1.Secret
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		secret, err = client.CoreV1().Secrets("kube-bind").Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return err
		}
		bs, found := secret.Data["kubeconfig"]
		if !found {
			return fmt.Errorf("secret %s/%s does not contain a kubeconfig", "kube-bind", name)
		}
		existingHost, existingNamespace, err := ParseRemoteKubeconfig(bs)
		if err != nil {
			return err
		}
		if existingHost != remoteHost || existingNamespace != remoteNamespace {
			return errors.NewAlreadyExists(corev1.Resource("secret"), secret.Name)
		}
		secret.Data["kubeconfig"] = []byte(kubeconfig)
		if _, err := client.CoreV1().Secrets("kube-bind").Update(ctx, secret, v1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	return secret, false, nil
}

// LoadRestConfigFromFile loads a kubeconfig string and returns a rest.Config
func LoadRestConfigFromFile(kubeconfig string) (*rest.Config, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}
	return config, nil
}

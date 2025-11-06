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

package plugin

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
)

func (b *BindAPIServiceOptions) createKubeconfigSecret(ctx context.Context, config *rest.Config, remoteHost, remoteNamespace, kubeconfig string) (string, error) {
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return "", err
	}

	// create kube-bind namespace
	if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind",
		},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", err
	} else if err == nil {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "📦 Created kube-binding namespace.\n")
	}

	// look for secret of the given identity
	secretName, err := base.FindRemoteKubeconfig(ctx, kubeClient, remoteNamespace, remoteHost)
	if err != nil {
		return "", err
	} else if secretName != "" {
		return secretName, nil
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "Creating secret for host %s, namespace %s\n", remoteHost, remoteNamespace)
	secretName, err = b.ensureKubeconfigSecretWithLogging(ctx, kubeconfig, "", kubeClient)
	if err != nil {
		return "", err
	}

	return secretName, nil
}

func (b *BindAPIServiceOptions) ensureKubeconfigSecretWithLogging(ctx context.Context, kubeconfig, name string, client kubeclient.Interface) (string, error) {
	secret, created, err := base.EnsureKubeconfigSecret(ctx, kubeconfig, name, client)
	if err != nil {
		return "", err
	}

	remoteHost, remoteNamespace, err := base.ParseRemoteKubeconfig([]byte(kubeconfig))
	if err != nil {
		return "", err
	}

	if b.remoteKubeconfigFile != "" {
		if created {
			fmt.Fprintf(b.Options.ErrOut, "🔒 Created secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
		} else {
			fmt.Fprintf(b.Options.ErrOut, "🔒 Updated secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
		}
	}

	return secret.Name, nil
}

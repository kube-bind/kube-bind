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
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"

	"github.com/kube-bind/kube-bind/deploy/konnector"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/authenticator"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin/resources"
)

// BindOptions contains the options for creating an APIBinding.
type BindOptions struct {
	*base.Options

	// url is the argument accepted by the command. It contains the
	// reference to where an APIService exists.
	url string
}

// NewBindOptions returns new BindOptions.
func NewBindOptions(streams genericclioptions.IOStreams) *BindOptions {
	return &BindOptions{
		Options: base.NewOptions(streams),
	}
}

// BindFlags binds fields to cmd's flagset.
func (b *BindOptions) BindFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
}

// Complete ensures all fields are initialized.
func (b *BindOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.url = args[0]
	}
	return nil
}

// Validate validates the BindOptions are complete and usable.
func (b *BindOptions) Validate() error {
	if b.url == "" {
		return errors.New("url is required as an argument") // should not happen because we validate that before
	}

	if _, err := url.Parse(b.url); err != nil {
		return fmt.Errorf("invalid url %q: %w", b.url, err)
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindOptions) Run(ctx context.Context) error {
	config, err := b.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return err
	}

	var group, resource, clusterID, kubeConfig, exportName string
	auth, err := authenticator.NewDefaultAuthenticator(10*time.Minute, func(ctx context.Context, sessionID, kcfg, r, g, id, export string) error {
		group = g
		resource = r
		id = id
		kubeConfig = kcfg
		exportName = export

		return nil
	})
	if err != nil {
		return err
	}

	exportURL, err := url.Parse(b.url)
	if err != nil {
		return err // should never happen because we test this in Validate()
	}

	provider, err := fetchAuthenticationRoute(exportURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch authentication url: %v", err)
	}

	sessionID := rand.String(rand.IntnRange(20, 30))

	if err := authenticate(provider, auth.Endpoint(ctx), sessionID); err != nil {
		return err
	}

	if err := auth.Execute(ctx); err != nil {
		return err
	}

	// bootstrap the konnector
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, sets.NewString()); err != nil {
		return err
	}

	// create the binding
	if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind",
		},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	secrets, err := kubeClient.CoreV1().Secrets("kube-bind").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var secretName string
	for _, s := range secrets.Items {
		if s.Annotations[resources.ClusterIDAnnotationKey] == clusterID {
			fmt.Printf("Found existing secret kube-bind/%s for identity %s\n", s.Name, clusterID)
			secretName = s.Name
			break
		}
	}
	if secretName == "" {
		fmt.Printf("Creating secret kube-bind/%s for identity %s\n", clusterID, clusterID)
		secretName, err = resources.EnsureServiceBindingAuthData(ctx, sessionID, clusterID, kubeConfig, "kube-bind", kubeClient)
		if err != nil {
			return err
		}
	}
	_, err = bindClient.KubeBindV1alpha1().APIServiceBindings().Create(ctx, &kubebindv1alpha1.APIServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resource + "." + group,
			Namespace: "kube-bind",
		},
		Spec: kubebindv1alpha1.APIServiceBindingSpec{
			KubeconfigSecretRef: kubebindv1alpha1.ClusterSecretKeyRef{
				LocalSecretKeyRef: kubebindv1alpha1.LocalSecretKeyRef{
					Name: secretName,
					Key:  "kubeconfig",
				},
				Namespace: "kube-bind",
			},
			Export: exportName,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

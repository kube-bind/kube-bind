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
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"
	clientgoversion "k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/deploy/konnector"
	"github.com/kube-bind/kube-bind/pkg/version"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

const (
	konnectorImage = "ghcr.io/kube-bind/konnector"
)

//nolint:unused
func (b *BindAPIServiceOptions) deployKonnector(ctx context.Context, config *rest.Config) error {
	logger := klog.FromContext(ctx)

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}

	bindVersion, err := version.BinaryVersion(clientgoversion.Get().GitVersion)
	if err != nil {
		return err
	}

	if b.KonnectorImageOverride != "" {
		fmt.Fprintf(b.Options.ErrOut, "🚀 Deploying konnector %s to namespace kube-bind with custom image %q.\n", bindVersion, b.KonnectorImageOverride) //nolint:errcheck
		if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, b.KonnectorImageOverride); err != nil {
			return err
		}
	} else if !b.SkipKonnector {
		konnectorVersion, installed, err := currentKonnectorVersion(ctx, kubeClient)
		if err != nil {
			return fmt.Errorf("failed to check current konnector version in the cluster: %w", err)
		}

		konnectorImage := fmt.Sprintf("%s:%s", konnectorImage, bindVersion)

		if installed && (konnectorVersion == "unknown" || konnectorVersion == "latest") {
			fmt.Fprintf(b.Options.ErrOut, "ℹ️ konnector of %s version already installed, skipping\n", konnectorVersion) //nolint:errcheck
			// fall through to CRD test
		} else if installed {
			konnectorSemVer, err := semver.Parse(strings.TrimLeft(konnectorVersion, "v"))
			if err != nil {
				return fmt.Errorf("failed to parse konnector SemVer version %q: %w", konnectorVersion, err)
			}
			bindSemVer, err := semver.Parse(strings.TrimLeft(bindVersion, "v"))
			if err != nil {
				return fmt.Errorf("failed to parse kubectl-bind SemVer version %q: %w", bindVersion, err)
			}
			if bindSemVer.GT(konnectorSemVer) {
				fmt.Fprintf(b.Options.ErrOut, "🚀 Updating konnector from %s to %s.\n", konnectorVersion, bindVersion) //nolint:errcheck
				if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, konnectorImage); err != nil {
					return err
				}
			} else if bindSemVer.LT(konnectorSemVer) {
				fmt.Fprintf(b.Options.ErrOut, "⚠️ Newer konnector %s installed. To downgrade to %s use --downgrade-konnector.\n", konnectorVersion, bindVersion) //nolint:errcheck
			}
		} else {
			fmt.Fprintf(b.Options.ErrOut, "🚀 Deploying konnector %s to namespace kube-bind.\n", bindVersion) //nolint:errcheck
			if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, konnectorImage); err != nil {
				return err
			}
		}
	}
	first := true
	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := bindClient.KubeBindV1alpha2().APIServiceBindings().List(ctx, metav1.ListOptions{})
		if err == nil {
			if !first {
				fmt.Fprintln(b.Options.IOStreams.ErrOut) //nolint:errcheck
			}
			return true, nil
		}

		logger.V(2).Info("Waiting for APIServiceBindings to be served", "error", err, "host", bindClient.RESTClient())
		if first {
			fmt.Fprint(b.Options.IOStreams.ErrOut, "   Waiting for the konnector to be ready") //nolint:errcheck
			first = false
		} else {
			fmt.Fprint(b.Options.IOStreams.ErrOut, ".") //nolint:errcheck
		}
		return false, nil
	})
}

func currentKonnectorVersion(ctx context.Context, kubeClient kubeclient.Interface) (string, bool, error) {
	deployment, err := kubeClient.AppsV1().Deployments("kube-bind").Get(ctx, "konnector", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", false, err
	} else if errors.IsNotFound(err) {
		return "", false, nil
	}

	img := deployment.Spec.Template.Spec.Containers[0].Image
	if !strings.HasPrefix(img, konnectorImage+":") {
		return "unknown", true, nil
	}

	version := strings.TrimPrefix(img, konnectorImage+":")
	return version, true, nil
}

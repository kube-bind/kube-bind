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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
)

// BindAPIServiceOptions are the options for the kubectl-bind-apiservice command.
type BindAPIServiceOptions struct {
	Options *base.Options
	Logs    *logs.Options

	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	OutputFormat       string
	Print              *genericclioptions.PrintFlags

	remoteKubeconfigFile      string
	remoteKubeconfigNamespace string
	remoteKubeconfigName      string
	remoteNamespace           string
	file                      string

	// skipKonnector skips the deployment of the konnector.
	SkipKonnector          bool
	KonnectorImageOverride string
	DowngradeKonnector     bool
	NoBanner               bool

	url string
}

// NewBindAPIServiceOptions returns new BindAPIServiceOptions.
func NewBindAPIServiceOptions(streams genericclioptions.IOStreams) *BindAPIServiceOptions {
	return &BindAPIServiceOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("kubectl-bind-apiservice"),
	}
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *BindAPIServiceOptions) AddCmdFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)

	cmd.Flags().StringVar(&b.remoteKubeconfigFile, "remote-kubeconfig", b.remoteKubeconfigFile, "A file path for a kubeconfig file to connect to the service provider cluster")
	cmd.Flags().StringVar(&b.remoteKubeconfigNamespace, "remote-kubeconfig-namespace", b.remoteKubeconfigNamespace, "The namespace of the remote kubeconfig secret to read from")
	cmd.Flags().StringVar(&b.remoteKubeconfigName, "remote-kubeconfig-name", b.remoteKubeconfigNamespace, "The name of the remote kubeconfig secret to read from")
	cmd.Flags().StringVarP(&b.file, "file", "f", b.file, "A file with an APIServiceExportRequest manifest. Use - to read from stdin")
	cmd.Flags().StringVar(&b.remoteNamespace, "remote-namespace", b.remoteNamespace, "The namespace in the remote cluster where the konnector is deployed")
	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", b.SkipKonnector, "Skip the deployment of the konnector")
	cmd.Flags().BoolVar(&b.DowngradeKonnector, "downgrade-konnector", b.DowngradeKonnector, "Downgrade the konnector to the version of the kubectl-bind-apiservice binary")
	cmd.Flags().StringVar(&b.KonnectorImageOverride, "konnector-image", b.KonnectorImageOverride, "The konnector image to use")
	cmd.Flags().MarkHidden("konnector-image") // nolint:errcheck
	cmd.Flags().BoolVar(&b.NoBanner, "no-banner", b.NoBanner, "Do not show the red banner")
	cmd.Flags().MarkHidden("no-banner") // nolint:errcheck
}

// Complete ensures all fields are initialized.
func (b *BindAPIServiceOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.url = args[0]
	}
	return nil
}

// Validate validates the BindAPIServiceOptions are complete and usable.
func (b *BindAPIServiceOptions) Validate() error {
	if b.url == "" && b.file == "" {
		return errors.New("url or file is required")
	}
	if b.url != "" && b.file != "" {
		return errors.New("url and file are mutually exclusive")
	}
	if b.url != "" {
		if _, err := url.Parse(b.url); err != nil {
			return fmt.Errorf("invalid url %q: %w", b.url, err)
		}
	}

	if allowed := sets.NewString(b.Print.AllowedFormats()...); *b.Print.OutputFormat != "" && !allowed.Has(*b.Print.OutputFormat) {
		return fmt.Errorf("invalid output format %q (allowed: %s)", *b.Print.OutputFormat, strings.Join(allowed.List(), ", "))
	}

	if (b.remoteKubeconfigNamespace == "" && b.remoteKubeconfigName != "") ||
		(b.remoteKubeconfigNamespace != "" && b.remoteKubeconfigName == "") {
		return errors.New("remote-kubeconfig-namespace and remote-kubeconfig-name must be specified together")
	}
	if b.remoteKubeconfigFile == "" && b.remoteKubeconfigNamespace == "" && b.remoteKubeconfigName == "" {
		return errors.New("remote-kubeconfig or remote-kubeconfig-namespace and remote-kubeconfig-name are required")
	}
	if b.file != "" && b.url != "" {
		return errors.New("file and arguments are mutually exclusive")
	}
	if b.file == "" && b.url == "" {
		return errors.New("file or arguments are required")
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindAPIServiceOptions) Run(ctx context.Context) error {
	// nolint: staticcheck
	config, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	remoteKubeconfig, remoteNamespace, remoteConfig, err := b.getRemoteKubeconfig(ctx, config)
	if err != nil {
		return err
	}
	bs, err := b.getRequestManifest()
	if err != nil {
		return err
	}
	request, err := b.unmarshalManifest(bs)
	if err != nil {
		return err
	}
	result, err := b.createServiceExportRequest(ctx, remoteConfig, remoteNamespace, request)
	if err != nil {
		return err
	}
	if err := b.deployKonnector(ctx, config); err != nil {
		return err
	}
	secretName, err := b.createKubeconfigSecret(ctx, config, remoteConfig.Host, remoteNamespace, remoteKubeconfig)
	if err != nil {
		return err
	}
	bindings, err := b.createAPIServiceBindings(ctx, config, result, secretName)
	if err != nil {
		return err
	}

	fmt.Fprintln(b.Options.ErrOut) // nolint: errcheck
	return b.printTable(ctx, config, bindings)
}

func (b *BindAPIServiceOptions) getRemoteKubeconfig(ctx context.Context, config *rest.Config) (kubeconfig, ns string, remoteConfig *rest.Config, err error) {
	var remoteKubeConfig *clientcmdapi.Config
	if b.remoteKubeconfigFile != "" {
		remoteKubeConfig, err = clientcmd.LoadFromFile(b.remoteKubeconfigFile)
		if err != nil {
			return "", "", nil, err
		}
	} else {
		kubeClient, err := kubeclient.NewForConfig(config)
		if err != nil {
			return "", "", nil, err
		}
		secret, err := kubeClient.CoreV1().Secrets(b.remoteKubeconfigNamespace).Get(ctx, b.remoteKubeconfigName, metav1.GetOptions{})
		if err != nil {
			return "", "", nil, err
		}
		bs, found := secret.Data["kubeconfig"]
		if !found {
			return "", "", nil, fmt.Errorf("secret %s/%s does not contain a kubeconfig", b.remoteKubeconfigNamespace, b.remoteKubeconfigName)
		}
		remoteKubeConfig, err = clientcmd.Load(bs)
		if err != nil {
			return "", "", nil, err
		}
	}

	c, found := remoteKubeConfig.Contexts[remoteKubeConfig.CurrentContext]
	if !found {
		return "", "", nil, fmt.Errorf("current context %q not found in remote kubeconfig", remoteKubeConfig.CurrentContext)
	}
	if b.remoteNamespace != "" {
		c.Namespace = b.remoteNamespace
	}
	if c.Namespace == "" {
		return "", "", nil, fmt.Errorf("remote namespace is required, either as flag or in the passed remote kubeconfig")
	}
	remoteConfig, err = clientcmd.NewDefaultClientConfig(*remoteKubeConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return "", "", nil, err
	}

	remoteKubeconfig, err := clientcmd.Write(*remoteKubeConfig)
	if err != nil {
		return "", "", nil, err
	}
	return string(remoteKubeconfig), c.Namespace, remoteConfig, nil
}

func (b *BindAPIServiceOptions) getRequestManifest() ([]byte, error) {
	if b.url != "" {
		resp, err := http.Get(b.url)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", b.url, err)
		}
		defer resp.Body.Close() // nolint: errcheck
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return body, nil
	} else if b.file == "-" {
		body, err := io.ReadAll(b.Options.IOStreams.In)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return body, nil
	}

	body, err := os.ReadFile(b.file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", b.file, err)
	}
	return body, nil
}

func (b *BindAPIServiceOptions) unmarshalManifest(bs []byte) (*kubebindv1alpha1.APIServiceExportRequest, error) {
	var request kubebindv1alpha1.APIServiceExportRequest
	if err := yaml.Unmarshal(bs, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	if request.APIVersion != kubebindv1alpha1.SchemeGroupVersion.String() {
		return nil, fmt.Errorf("invalid apiVersion %q", request.APIVersion)
	}
	if request.Kind != "APIServiceExportRequest" {
		return nil, fmt.Errorf("invalid kind %q", request.Kind)
	}
	return &request, nil
}

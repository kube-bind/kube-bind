/*
Copyright 2025 The Kube Bind Authors.

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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	bindapiservice "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/plugin"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// BindDeployOptions are the options for the kubectl-bind-deploy command.
type BindDeployOptions struct {
	*base.Options
	Logs *logs.Options

	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	OutputFormat       string
	Print              *genericclioptions.PrintFlags
	printer            printers.ResourcePrinter

	// secretRef is the namespace/name reference to a secret containing the BindingResourceResponse
	secretRef string
	// secretKey is the key in the secret containing the BindingResourceResponse
	secretKey string
	// file is the path to a file containing the BindingResourceResponse
	file string

	// skipKonnector skips the deployment of the konnector.
	SkipKonnector          bool
	KonnectorImageOverride string
	DowngradeKonnector     bool
	// KonnectorHostAlias is a list of host alias entries to add to the konnector pods.
	KonnectorHostAlias []string
	// KonnectorHostAliasParsed is the parsed version of KonnectorHostAlias.
	KonnectorHostAliasParsed []corev1.HostAlias

	NoBanner bool
	DryRun   bool
}

// NewBindDeployOptions returns new BindDeployOptions.
func NewBindDeployOptions(streams genericclioptions.IOStreams) *BindDeployOptions {
	return &BindDeployOptions{
		Options:   base.NewOptions(streams),
		Logs:      logs.NewOptions(),
		Print:     genericclioptions.NewPrintFlags("kubectl-bind-deploy"),
		secretKey: "kubeconfig", // default key
	}
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *BindDeployOptions) AddCmdFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)

	// Secret/file source
	cmd.Flags().StringVar(&b.secretRef, "secret", b.secretRef, "Reference to a secret containing the BindingResourceResponse (format: namespace/name)")
	cmd.Flags().StringVar(&b.secretKey, "secret-key", b.secretKey, "Key in the secret containing the BindingResourceResponse (default: kubeconfig)")
	cmd.Flags().StringVarP(&b.file, "file", "f", b.file, "A file with a BindingResourceResponse manifest. Use - to read from stdin")

	// Konnector configuration
	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", b.SkipKonnector, "Skip the deployment of the konnector")
	cmd.Flags().BoolVarP(&b.DryRun, "dry-run", "d", b.DryRun, "If true, only print the actions that would be taken without executing them")
	cmd.Flags().StringVar(&b.KonnectorImageOverride, "konnector-image", b.KonnectorImageOverride, "The konnector image to use")
	cmd.Flags().MarkHidden("konnector-image") //nolint:errcheck
	cmd.Flags().StringSliceVarP(&b.KonnectorHostAlias, "konnector-host-alias", "", []string{}, "Add a host alias to the konnector pods in the format IP:hostname1,hostname2")
	cmd.Flags().MarkHidden("konnector-host-alias") //nolint:errcheck
	cmd.Flags().BoolVar(&b.DowngradeKonnector, "downgrade-konnector", b.DowngradeKonnector, "Downgrade the konnector to the version of the kubectl-bind binary")
	cmd.Flags().BoolVar(&b.NoBanner, "no-banner", b.NoBanner, "Do not show the red banner")
	cmd.Flags().MarkHidden("no-banner") //nolint:errcheck
}

// Complete ensures all fields are initialized.
func (b *BindDeployOptions) Complete(args []string) error {
	if err := b.Options.Complete(false); err != nil {
		return err
	}

	printer, err := b.Print.ToPrinter()
	if err != nil {
		return err
	}
	b.printer = printer

	// Parse konnector host alias entries
	for _, hostAlias := range b.KonnectorHostAlias {
		parts := strings.SplitN(hostAlias, ":", 2)
		if len(parts) != 2 {
			continue
		}
		hostnames := strings.Split(parts[1], ",")
		b.KonnectorHostAliasParsed = append(b.KonnectorHostAliasParsed, corev1.HostAlias{
			IP:        parts[0],
			Hostnames: hostnames,
		})
	}
	return nil
}

// Validate validates the BindDeployOptions are complete and usable.
func (b *BindDeployOptions) Validate() error {
	if b.secretRef == "" && b.file == "" {
		return errors.New("either --secret or --file is required")
	}

	if b.secretRef != "" && b.file != "" {
		return errors.New("only one of --secret or --file can be specified")
	}

	if b.secretRef != "" {
		parts := strings.SplitN(b.secretRef, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid --secret format %q, expected namespace/name", b.secretRef)
		}
	}

	// Validate konnector host alias entries
	for _, hostAlias := range b.KonnectorHostAlias {
		parts := strings.SplitN(hostAlias, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid konnector-host-alias entry %q, expected format IP:hostname1,hostname2", hostAlias)
		}
		if parts[0] == "" {
			return fmt.Errorf("invalid konnector-host-alias entry %q, IP address is empty", hostAlias)
		}
		if parts[1] == "" {
			return fmt.Errorf("invalid konnector-host-alias entry %q, hostnames are empty", hostAlias)
		}
	}

	return b.Options.Validate()
}

// Run starts the deployment process.
func (b *BindDeployOptions) Run(ctx context.Context) error {
	fmt.Fprintf(b.Options.ErrOut, "Starting deployment process...\n")

	config, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	// Get the BindingResourceResponse
	response, err := b.getBindingResponse(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to get BindingResourceResponse: %w", err)
	}

	// Use the shared binder to deploy
	binderOpts := &bindapiservice.BinderOptions{
		IOStreams:                b.Options.IOStreams,
		SkipKonnector:            b.SkipKonnector,
		KonnectorImageOverride:   b.KonnectorImageOverride,
		KonnectorHostAliasParsed: b.KonnectorHostAliasParsed,
		DowngradeKonnector:       b.DowngradeKonnector,
		DryRun:                   b.DryRun,
	}
	binder := bindapiservice.NewBinder(config, binderOpts)

	bindings, err := b.bindFromKubeconfig(ctx, binder, config, response)
	if err != nil {
		return fmt.Errorf("failed to create bindings: %w", err)
	}

	fmt.Fprintln(b.Options.ErrOut)
	return b.printTable(ctx, config, bindings)
}

// getBindingResponse reads the BindingResourceResponse from the specified source.
func (b *BindDeployOptions) getBindingResponse(ctx context.Context, config *rest.Config) (*kubebindv1alpha2.BindingResourceResponse, error) {
	var data []byte
	var err error

	if b.secretRef != "" {
		parts := strings.SplitN(b.secretRef, "/", 2)
		namespace, name := parts[0], parts[1]

		kubeClient, err := kubeclient.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create kube client: %w", err)
		}

		secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
		}

		data, ok := secret.Data[b.secretKey]
		if !ok {
			return nil, fmt.Errorf("secret %s/%s does not contain key %q", namespace, name, b.secretKey)
		}

		var response kubebindv1alpha2.BindingResourceResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal BindingResourceResponse from secret: %w", err)
		}

		fmt.Fprintf(b.Options.ErrOut, "📦 Read BindingResourceResponse from secret %s/%s\n", namespace, name)
		return &response, nil
	}

	// Read from file
	if b.file == "-" {
		data, err = io.ReadAll(b.Options.In)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(b.file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", b.file, err)
		}
	}

	var response kubebindv1alpha2.BindingResourceResponse
	if err := yaml.Unmarshal(data, &response); err != nil {
		// Try JSON if YAML fails
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal BindingResourceResponse: %w", err)
		}
	}

	if b.file == "-" {
		fmt.Fprintf(b.Options.ErrOut, "📦 Read BindingResourceResponse from stdin\n")
	} else {
		fmt.Fprintf(b.Options.ErrOut, "📦 Read BindingResourceResponse from file %s\n", b.file)
	}

	return &response, nil
}

// bindFromKubeconfig creates bindings from a BindingResourceResponse that contains only kubeconfig.
func (b *BindDeployOptions) bindFromKubeconfig(
	ctx context.Context,
	binder *bindapiservice.Binder,
	config *rest.Config,
	response *kubebindv1alpha2.BindingResourceResponse,
) ([]*kubebindv1alpha2.APIServiceBinding, error) {
	// Ensure client side namespace exists
	err := b.ensureClientSideNamespaceExists(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure kube-bind namespace exists: %w", err)
	}

	// Parse remote kubeconfig to get host and namespace
	remoteHost, remoteNamespace, err := base.ParseRemoteKubeconfig(response.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Find or create the kubeconfig secret
	secretName, err := base.FindRemoteKubeconfig(ctx, kubeClient, remoteNamespace, remoteHost)
	if err != nil {
		return nil, err
	}

	secret, created, err := base.EnsureKubeconfigSecret(ctx, string(response.Kubeconfig), secretName, kubeClient)
	if err != nil {
		return nil, err
	}

	if created {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "🔒 Created secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	} else {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "🔒 Updated secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	}

	if b.DryRun {
		return nil, nil
	}

	// Load remote config
	remoteKubeConfig, err := clientcmd.Load(response.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load remote kubeconfig: %w", err)
	}

	_, err = clientcmd.NewDefaultClientConfig(*remoteKubeConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create remote config: %w", err)
	}

	// Deploy konnector
	if !b.SkipKonnector {
		if err := b.deployKonnector(ctx, config); err != nil {
			return nil, fmt.Errorf("failed to deploy konnector: %w", err)
		}
	}

	// If response has requests, process them (for full BindingResourceResponse flow)
	if len(response.Requests) > 0 {
		return binder.BindFromResponse(ctx, response)
	}

	return []*kubebindv1alpha2.APIServiceBinding{}, nil
}

func (b *BindDeployOptions) ensureClientSideNamespaceExists(ctx context.Context, config *rest.Config) error {
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, "kube-bind", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-bind",
			},
		}
		if _, err = kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "📦 Created kube-bind namespace.\n")
	}
	return nil
}

func (b *BindDeployOptions) deployKonnector(ctx context.Context, config *rest.Config) error {
	tempOpts := &bindapiservice.BindAPIServiceOptions{
		Options:                  b.Options,
		SkipKonnector:            b.SkipKonnector,
		KonnectorImageOverride:   b.KonnectorImageOverride,
		DowngradeKonnector:       b.DowngradeKonnector,
		DryRun:                   b.DryRun,
		KonnectorHostAliasParsed: b.KonnectorHostAliasParsed,
	}
	return tempOpts.DeployKonnector(ctx, config)
}

func (b *BindDeployOptions) printTable(ctx context.Context, config *rest.Config, bindings []*kubebindv1alpha2.APIServiceBinding) error {
	if len(bindings) == 0 {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "No bindings created.\n")
		return nil
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "\nCreated bindings:\n")
	for _, binding := range bindings {
		fmt.Fprintf(b.Options.IOStreams.Out, "  - %s\n", binding.Name)
	}

	return nil
}

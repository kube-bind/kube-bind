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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// BindAPIServiceOptions are the options for the kubectl-bind-apiservice command.
type BindAPIServiceOptions struct {
	*base.Options
	Logs *logs.Options

	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	OutputFormat       string
	Print              *genericclioptions.PrintFlags
	printer            printers.ResourcePrinter

	remoteKubeconfigFile      string
	remoteKubeconfigNamespace string
	remoteKubeconfigName      string
	remoteNamespace           string
	file                      string
	FromDryRun                string

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
	Template string
	Name     string
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

	cmd.Flags().StringVar(&b.Template, "template-name", b.Template, "A template name to use for binding")
	cmd.Flags().StringVar(&b.Name, "name", b.Name, "The name of the BindableResourcesRequest to create")
	cmd.Flags().StringVar(&b.remoteKubeconfigFile, "remote-kubeconfig", b.remoteKubeconfigFile, "A file path for a kubeconfig file to connect to the service provider cluster")
	cmd.Flags().StringVar(&b.remoteKubeconfigNamespace, "remote-kubeconfig-namespace", b.remoteKubeconfigNamespace, "The namespace of the remote kubeconfig secret to read from")
	cmd.Flags().StringVar(&b.remoteKubeconfigName, "remote-kubeconfig-name", b.remoteKubeconfigNamespace, "The name of the remote kubeconfig secret to read from")
	cmd.Flags().StringVarP(&b.file, "file", "f", b.file, "A file with an APIServiceExportRequest manifest. Use - to read from stdin")
	cmd.Flags().StringVar(&b.remoteNamespace, "remote-namespace", b.remoteNamespace, "The namespace in the remote cluster where the konnector is deployed")
	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", b.SkipKonnector, "Skip the deployment of the konnector")
	cmd.Flags().BoolVar(&b.DowngradeKonnector, "downgrade-konnector", b.DowngradeKonnector, "Downgrade the konnector to the version of the kubectl-bind-apiservice binary")
	cmd.Flags().StringVar(&b.KonnectorImageOverride, "konnector-image", b.KonnectorImageOverride, "The konnector image to use")
	cmd.Flags().BoolVarP(&b.DryRun, "dry-run", "d", b.DryRun, "If true, only print the requests that would be sent to the service provider after authentication, without creating any resources on the consumer cluster.")
	cmd.Flags().StringVar(&b.FromDryRun, "from-dry-run", b.FromDryRun, "Apply bindings from a previous dry-run session using the session ID)")
	cmd.Flags().MarkHidden("konnector-image") //nolint:errcheck
	cmd.Flags().BoolVar(&b.NoBanner, "no-banner", b.NoBanner, "Do not show the red banner")
	cmd.Flags().MarkHidden("no-banner") //nolint:errcheck
	cmd.Flags().StringSliceVarP(&b.KonnectorHostAlias, "konnector-host-alias", "", []string{}, "Add a host alias to the konnector pods in the format IP:hostname1,hostname2")
	cmd.Flags().MarkHidden("konnector-host-alias") //nolint:errcheck
}

// Complete ensures all fields are initialized.
func (b *BindAPIServiceOptions) Complete(args []string) error {
	if len(args) > 0 {
		b.Name = args[0]
	}
	if err := b.Options.Complete(false); err != nil {
		return err
	}

	printer, err := b.Print.ToPrinter()
	if err != nil {
		return err
	}

	b.printer = printer

	// parse konnector host alias entries
	for _, hostAlias := range b.KonnectorHostAlias {
		parts := strings.SplitN(hostAlias, ":", 2)
		hostnames := strings.Split(parts[1], ",")
		b.KonnectorHostAliasParsed = append(b.KonnectorHostAliasParsed, corev1.HostAlias{
			IP:        parts[0],
			Hostnames: hostnames,
		})
	}
	return nil
}

// Validate validates the BindAPIServiceOptions are complete and usable.
func (b *BindAPIServiceOptions) Validate() error {
	if b.file == "" && b.Template == "" && b.FromDryRun == "" {
		return errors.New("file, template-name or --from-dry-run are required")
	}

	if allowed := sets.NewString(b.Print.AllowedFormats()...); *b.Print.OutputFormat != "" && !allowed.Has(*b.Print.OutputFormat) {
		return fmt.Errorf("invalid output format %q (allowed: %s)", *b.Print.OutputFormat, strings.Join(allowed.List(), ", "))
	}

	if b.FromDryRun == "" && b.Template == "" {
		if (b.remoteKubeconfigNamespace == "" && b.remoteKubeconfigName != "") ||
			(b.remoteKubeconfigNamespace != "" && b.remoteKubeconfigName == "") {
			return errors.New("remote-kubeconfig-namespace and remote-kubeconfig-name must be specified together")
		}
		if b.remoteKubeconfigFile == "" && b.remoteKubeconfigNamespace == "" && b.remoteKubeconfigName == "" {
			return errors.New("remote-kubeconfig or remote-kubeconfig-namespace and remote-kubeconfig-name are required")
		}
	}
	if b.Name == "" && b.file == "" && b.FromDryRun == "" && b.Template == "" {
		return errors.New("name is required")
	}

	// TODO(mjudeikis): This code is duplicate fromn bind/plugin/bind.go. Unify.
	// validate konnector host alias entries
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

// Run starts the binding process.
func (b *BindAPIServiceOptions) Run(ctx context.Context) error {
	fmt.Fprintf(b.Options.ErrOut, "Starting binding process...\n")

	config, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	// Use the shared binder to create bindings
	binderOpts := &BinderOptions{
		IOStreams:                 b.Options.IOStreams,
		SkipKonnector:             b.SkipKonnector,
		KonnectorImageOverride:    b.KonnectorImageOverride,
		KonnectorHostAliasParsed:  b.KonnectorHostAliasParsed,
		DowngradeKonnector:        b.DowngradeKonnector,
		RemoteKubeconfigFile:      b.remoteKubeconfigFile,
		RemoteKubeconfigNamespace: b.remoteKubeconfigNamespace,
		RemoteKubeconfigName:      b.remoteKubeconfigName,
		RemoteNamespace:           b.remoteNamespace,
		File:                      b.file,
		DryRun:                    b.DryRun,
	}
	binder := NewBinder(config, binderOpts)

	var bindings []*kubebindv1alpha2.APIServiceBinding

	switch {
	case b.FromDryRun != "":
		bindings, err = binder.BindFromDryRun(ctx, b.FromDryRun)
		if err != nil {
			return fmt.Errorf("failed to apply dry-run assets: %w", err)
		}
	case b.Template != "":
		r, err := b.bindTemplate(ctx)
		if err != nil {
			return err
		}

		if b.DryRun {
			return b.handleTemplateDryRun(r.response)
		}

		bindings, err = binder.BindFromResponse(ctx, r.response)
		if err != nil {
			return fmt.Errorf("failed to create bindings: %w", err)
		}
	case b.file != "":
		if b.DryRun {
			return b.handleFileDryRun()
		}

		bindings, err = binder.BindFromFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create bindings: %w", err)
		}
	}

	fmt.Fprintln(b.Options.ErrOut)
	return b.printTable(ctx, config, bindings)
}

func (b *BindAPIServiceOptions) getRemoteKubeconfig(ctx context.Context, config *rest.Config, namespace, name string) (kubeconfig, ns string, remoteConfig *rest.Config, err error) {
	var remoteKubeConfig *clientcmdapi.Config

	switch {
	case b.remoteKubeconfigFile != "":
		remoteKubeConfig, err = clientcmd.LoadFromFile(b.remoteKubeconfigFile)
		if err != nil {
			return "", "", nil, err
		}
	case b.remoteKubeconfigNamespace != "" && b.remoteKubeconfigName != "":
		name = b.remoteKubeconfigName
		namespace = b.remoteKubeconfigNamespace
		fallthrough
	default:
		kubeClient, err := kubeclient.NewForConfig(config)
		if err != nil {
			return "", "", nil, err
		}
		secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", "", nil, err
		}
		bs, found := secret.Data["kubeconfig"]
		if !found {
			return "", "", nil, fmt.Errorf("secret %s/%s does not contain a kubeconfig", namespace, name)
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

func (b *BindAPIServiceOptions) ensureClientSideNamespaceExists(ctx context.Context, config *rest.Config) error {
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
		} else {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "Created kube-bind namespace.\n")
		}
	}
	return nil
}

type bindTemplateResult struct {
	response  *kubebindv1alpha2.BindingResourceResponse
	namespace string
	name      string
}

func (b *BindAPIServiceOptions) bindTemplate(ctx context.Context) (*bindTemplateResult, error) {
	config, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Get authenticated client with auto-login
	client, err := b.Options.GetAuthenticatedClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	name := b.Name
	if name == "" {
		name = b.Template
	}

	bindRequest := &kubebindv1alpha2.BindableResourcesRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		TemplateRef: kubebindv1alpha2.APIServiceExportTemplateRef{
			Name: b.Template,
		},
	}

	bindResponse, err := client.Bind(ctx, bindRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to template %q: %w", b.Template, err)
	}

	if bindResponse.Authentication.OAuth2CodeGrant == nil {
		return nil, fmt.Errorf("unexpected response: authentication.oauth2CodeGrant is nil")
	}

	if b.DryRun {
		return &bindTemplateResult{
			response:  bindResponse,
			namespace: "",
			name:      "",
		}, nil
	}

	err = b.ensureClientSideNamespaceExists(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure kube-bind namespace exists: %w", err)
	}

	// copy kubeconfig into local cluster
	remoteHost, remoteNamespace, err := base.ParseRemoteKubeconfig(bindResponse.Kubeconfig)
	if err != nil {
		return nil, err
	}
	secretName, err := base.FindRemoteKubeconfig(ctx, kubeClient, remoteNamespace, remoteHost)
	if err != nil {
		return nil, err
	}
	secret, created, err := base.EnsureKubeconfigSecret(ctx, string(bindResponse.Kubeconfig), secretName, kubeClient)
	if err != nil {
		return nil, err
	}
	if created {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "🔒 Created secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	} else {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "🔒 Updated secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	}
	return &bindTemplateResult{
		response:  bindResponse,
		namespace: secret.Namespace,
		name:      secret.Name,
	}, nil
}

// handleTemplateDryRun prints the requests and saves assets locally without touching the consumer cluster
func (b *BindAPIServiceOptions) handleTemplateDryRun(response *kubebindv1alpha2.BindingResourceResponse) error {
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "Dry-run mode: outputting APIServiceExport requests\n\n")

	if response.Authentication.OAuth2CodeGrant == nil {
		return fmt.Errorf("authentication data missing in response")
	}
	sessionID := response.Authentication.OAuth2CodeGrant.SessionID

	if err := b.printRequests(response.Requests); err != nil {
		return err
	}

	if err := b.saveBindingAssets(response, sessionID); err != nil {
		return fmt.Errorf("failed to save binding assets: %w", err)
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "\n✅ Dry-run complete. No changes made to consumer cluster.\n")
	return nil
}

// saveBindingAssets saves the binding response to local storage for later use
func (b *BindAPIServiceOptions) saveBindingAssets(response *kubebindv1alpha2.BindingResourceResponse, sessionID string) error {
	if response.Authentication.OAuth2CodeGrant == nil {
		return fmt.Errorf("authentication data missing")
	}

	userID := response.Authentication.OAuth2CodeGrant.ID

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	dryRunDir := filepath.Join(homeDir, ".kube-bind", "dry-run", sessionID)
	if err := os.MkdirAll(dryRunDir, 0700); err != nil {
		return fmt.Errorf("failed to create dry-run directory: %w", err)
	}

	kubeconfigPath := filepath.Join(dryRunDir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, response.Kubeconfig, 0600); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	requestFiles := make([]string, 0, len(response.Requests))
	for i, request := range response.Requests {
		requestPath := filepath.Join(dryRunDir, fmt.Sprintf("request-%d.yaml", i))

		var obj map[string]interface{}
		if err := json.Unmarshal(request.Raw, &obj); err != nil {
			return fmt.Errorf("failed to unmarshal request %d: %w", i, err)
		}

		yamlData, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal request %d to YAML: %w", i, err)
		}

		if err := os.WriteFile(requestPath, yamlData, 0600); err != nil {
			return fmt.Errorf("failed to save request %d: %w", i, err)
		}

		requestFiles = append(requestFiles, requestPath)
	}

	assets := &base.DryRunAssets{
		SessionID:    sessionID,
		UserID:       userID,
		ServerURL:    b.ServerName,
		ClusterID:    b.ClusterName,
		CreatedAt:    time.Now(),
		Kubeconfig:   kubeconfigPath,
		RequestFiles: requestFiles,
	}

	metadataPath := filepath.Join(dryRunDir, "metadata.json")
	metadataData, err := json.MarshalIndent(assets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataData, 0600); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "\n💾 Dry-run assets saved to: %s\n", dryRunDir)
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "   Session ID: %s\n", sessionID)
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "   To apply later: kubectl bind apiservice --from-dry-run %s\n", sessionID)

	return nil
}

// handleFileDryRun prints the request from file without touching the consumer cluster
func (b *BindAPIServiceOptions) handleFileDryRun() error {
	fmt.Fprintf(b.Options.IOStreams.ErrOut, "Dry-run mode: outputting APIServiceExport request from file\n\n")

	var data []byte
	var err error
	if b.file == "-" {
		data, err = io.ReadAll(b.Options.IOStreams.In)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(b.file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", b.file, err)
		}
	}

	var request kubebindv1alpha2.APIServiceExportRequest
	if err := yaml.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	if err := b.printRequest(&request, data); err != nil {
		return err
	}

	fmt.Fprintf(b.Options.IOStreams.ErrOut, "\n✅ Dry-run complete. No changes made to consumer cluster.\n")
	return nil
}

// printRequest prints a single APIServiceExportRequest using the configured printer
func (b *BindAPIServiceOptions) printRequest(request *kubebindv1alpha2.APIServiceExportRequest, rawData []byte) error {
	if b.printer != nil {
		if err := b.printer.PrintObj(request, b.Options.IOStreams.Out); err != nil {
			return fmt.Errorf("failed to print request: %w", err)
		}
	} else {
		fmt.Fprintf(b.Options.IOStreams.Out, "%s", rawData)
	}
	return nil
}

// printRequests prints multiple requests from a response, handling YAML separators
func (b *BindAPIServiceOptions) printRequests(requests []runtime.RawExtension) error {
	for i, request := range requests {
		if i > 0 && b.Print.OutputFormat != nil && *b.Print.OutputFormat == "yaml" {
			fmt.Fprintf(b.Options.IOStreams.Out, "---\n")
		}

		var apiRequest kubebindv1alpha2.APIServiceExportRequest
		if err := json.Unmarshal(request.Raw, &apiRequest); err != nil {
			return fmt.Errorf("failed to unmarshal request %d: %w", i, err)
		}

		if err := b.printRequest(&apiRequest, request.Raw); err != nil {
			return fmt.Errorf("failed to print request %d: %w", i, err)
		}
	}
	return nil
}

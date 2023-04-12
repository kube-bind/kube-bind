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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/authenticator"
)

// BindOptions contains the options for creating an APIBinding.
type BindOptions struct {
	*base.Options
	Logs *logs.Options

	Print   *genericclioptions.PrintFlags
	printer printers.ResourcePrinter
	DryRun  bool

	// url is the argument accepted by the command. It contains the
	// reference to where an APIService exists.
	URL string

	// skipKonnector skips the deployment of the konnector.
	SkipKonnector bool

	// The konnector image to use and override default konnector image
	KonnectorImageOverride string

	// Runner is runs the command. It can be replaced in tests.
	Runner func(cmd *exec.Cmd) error

	flags *pflag.FlagSet
}

// NewBindOptions returns new BindOptions.
func NewBindOptions(streams genericclioptions.IOStreams) *BindOptions {
	opts := &BindOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("kubectl-bind").WithDefaultOutput("yaml"),

		Runner: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
	}

	return opts
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *BindOptions) AddCmdFlags(cmd *cobra.Command) {
	b.flags = cmd.Flags()

	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)

	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", b.SkipKonnector, "Skip the deployment of the konnector")
	cmd.Flags().BoolVarP(&b.DryRun, "dry-run", "d", b.DryRun, "If true, only print the requests that would be sent to the service provider after authentication, without actually binding.")
	cmd.Flags().StringVar(&b.KonnectorImageOverride, "konnector-image", b.KonnectorImageOverride, "The konnector image to use")
}

// Complete ensures all fields are initialized.
func (b *BindOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.URL = args[0]
	}

	printer, err := b.Print.ToPrinter()
	if err != nil {
		return err
	}

	b.printer = printer

	return nil
}

// Validate validates the BindOptions are complete and usable.
func (b *BindOptions) Validate() error {
	if b.URL == "" {
		return errors.New("url is required as an argument") // should not happen because we validate that before
	}

	if _, err := url.Parse(b.URL); err != nil {
		return fmt.Errorf("invalid url %q: %w", b.URL, err)
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindOptions) Run(ctx context.Context, urlCh chan<- string) error {
	config, err := b.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}

	exportURL, err := url.Parse(b.URL)
	if err != nil {
		return err // should never happen because we test this in Validate()
	}

	provider, err := getProvider(exportURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch authentication url %q: %v", exportURL, err)
	}

	if provider.APIVersion != kubebindv1alpha1.GroupVersion {
		return fmt.Errorf("unsupported binding provider version: %q", provider.APIVersion)
	}

	ns, err := kubeClient.CoreV1().Namespaces().Get(ctx, "kube-bind", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-bind",
			},
		}
		if ns, err = kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
			return err
		} else {
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "📦 Created kube-bind namespace.\n") // nolint: errcheck
		}
	}

	auth := authenticator.NewLocalhostCallbackAuthenticator()
	err = auth.Start()
	fmt.Fprintf(b.Options.ErrOut, "\n\n")
	if err != nil {
		return err
	}

	sessionID := SessionID()
	if err := b.authenticate(provider, auth.Endpoint(), sessionID, ClusterID(ns), urlCh); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	response, gvk, err := auth.WaitForResponse(timeoutCtx)
	if err != nil {
		return err
	}

	fmt.Fprintf(b.IOStreams.ErrOut, "🔑 Successfully authenticated to %s\n", exportURL.String()) // nolint: errcheck

	// verify the response
	if gvk.GroupVersion() != kubebindv1alpha1.SchemeGroupVersion || gvk.Kind != "BindingResponse" {
		return fmt.Errorf("unexpected response type %s, only supporting %s", gvk, kubebindv1alpha1.SchemeGroupVersion.WithKind("BindingResponse"))
	}
	bindingResponse, ok := response.(*kubebindv1alpha1.BindingResponse)
	if !ok {
		return fmt.Errorf("unexpected response type %T", response)
	}
	if bindingResponse.Authentication.OAuth2CodeGrant == nil {
		return fmt.Errorf("unexpected response: authentication.oauth2CodeGrant is nil")
	}
	if bindingResponse.Authentication.OAuth2CodeGrant.SessionID != sessionID {
		return fmt.Errorf("unexpected response: sessionID does not match")
	}

	// extract the requests
	var apiRequests []*kubebindv1alpha1.APIServiceExportRequestResponse
	for i, request := range bindingResponse.Requests {
		var meta metav1.TypeMeta
		if err := json.Unmarshal(request.Raw, &meta); err != nil {
			return fmt.Errorf("unexpected response: failed to unmarshal request #%d: %v", i, err)
		}
		if got, expected := meta.APIVersion, kubebindv1alpha1.SchemeGroupVersion.String(); got != expected {
			return fmt.Errorf("unexpected response: request #%d is not %s, got %s", i, expected, got)
		}
		var apiRequest kubebindv1alpha1.APIServiceExportRequestResponse
		if err := json.Unmarshal(request.Raw, &apiRequest); err != nil {
			return fmt.Errorf("failed to unmarshal api request #%d: %v", i+1, err)
		}
		apiRequests = append(apiRequests, &apiRequest)
	}

	// copy kubeconfig into local cluster
	remoteHost, remoteNamespace, err := base.ParseRemoteKubeconfig(bindingResponse.Kubeconfig)
	if err != nil {
		return err
	}
	secretName, err := base.FindRemoteKubeconfig(ctx, kubeClient, remoteNamespace, remoteHost)
	if err != nil {
		return err
	}
	secret, created, err := base.EnsureKubeconfigSecret(ctx, string(bindingResponse.Kubeconfig), secretName, kubeClient)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(b.Options.ErrOut, "🔒 Created secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	} else {
		fmt.Fprintf(b.Options.ErrOut, "🔒 Updated secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
	}

	// print the request in dry-run mode
	if b.DryRun {
		for _, request := range apiRequests {
			if err = b.printer.PrintObj(request, b.IOStreams.Out); err != nil {
				return err
			}
		}
	}

	if b.DryRun {
		return nil
	}

	// call sub-command for apiservices
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	for _, request := range apiRequests {
		bs, err := json.Marshal(request)
		if err != nil {
			return err
		}

		args := []string{
			"apiservice",
			"--remote-kubeconfig-namespace", secret.Namespace,
			"--remote-kubeconfig-name", secret.Name,
			"-f", "-",
		}
		b.flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Changed && PassOnFlags.Has(flag.Name) {
				args = append(args, "--"+flag.Name+"="+flag.Value.String())
			}
		})

		if b.KonnectorImageOverride != "" {
			args = append(args, "--konnector-image"+"="+b.KonnectorImageOverride)
		}

		// TODO: support passing through the base options

		fmt.Fprintf(b.Options.ErrOut, "🚀 Executing: %s %s\n", "kubectl bind", strings.Join(args, " ")) // nolint: errcheck
		fmt.Fprintf(b.Options.ErrOut, "✨ Use \"-o yaml\" and \"--dry-run\" to get the APIServiceExportRequest.\n   and pass it to \"kubectl bind apiservice\" directly. Great for automation.\n")
		command := exec.CommandContext(ctx, executable, append(args, "--no-banner")...)
		command.Stdin = bytes.NewReader(bs)
		command.Stdout = b.Options.Out
		command.Stderr = b.Options.ErrOut
		if err := b.Runner(command); err != nil {
			return err
		}
	}

	return nil
}

func ClusterID(ns *corev1.Namespace) string {
	hash := sha256.Sum224([]byte(ns.UID))
	base62hash := toBase62(hash)
	return base62hash[:6] // 50 billion
}

func SessionID() string {
	var b [28]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return toBase62(b)[:6] // 50 billion
}

func toBase62(hash [28]byte) string {
	var i big.Int
	i.SetBytes(hash[:])
	return i.Text(62)
}

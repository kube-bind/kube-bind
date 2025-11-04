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

package webhook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	certManagerGroup   = "cert-manager.io"
	certManagerVersion = "v1"
	certManagerKind    = "Certificate"
	issuerKind         = "Issuer"

	webhookCertName      = "kube-bind-webhook-cert"
	webhookCertSecret    = "kube-bind-webhook-cert" //nolint:gosec // This is a secret name, not credentials
	webhookIssuerName    = "kube-bind-webhook-issuer"
	webhookCertNamespace = "kube-bind"
)

// EnsureWebhookCertificates uses cert-manager to generate certificates
func EnsureWebhookCertificates(ctx context.Context, cfg *rest.Config, kubeClient kubernetes.Interface, crClient client.Client, scheme *runtime.Scheme) error {
	logger := klog.FromContext(ctx)

	if CertsExist(WebhookCertDirectory) {
		logger.V(1).Info("Webhook certificates already exist, skipping generation", "certDir", WebhookCertDirectory)
		return nil
	}

	hasCertManager, err := checkCertManagerInstalled(ctx, cfg)
	if err != nil {
		logger.V(2).Info("Error checking cert-manager installation", "error", err)
		return err
	}
	if !hasCertManager {
		logger.V(2).Info("Cert-manager not installed, skipping certificate generation")
		return fmt.Errorf("cert-manager not installed")
	}

	logger.V(1).Info("Cert-manager detected, generating webhook certificates")

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookCertNamespace,
		},
	}
	if err := crClient.Get(ctx, types.NamespacedName{Name: webhookCertNamespace}, ns); err != nil {
		if err := crClient.Create(ctx, ns); err != nil {
			logger.V(1).Info("Failed to create namespace, may already exist", "error", err)
		}
	}

	if err := createSelfSignedIssuer(ctx, cfg, crClient); err != nil {
		return fmt.Errorf("failed to create issuer: %w", err)
	}

	if err := createCertificate(ctx, cfg, crClient); err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	logger.V(1).Info("Waiting for certificate to be ready")
	if err := waitForCertificateReady(ctx, cfg); err != nil {
		return fmt.Errorf("certificate not ready: %w", err)
	}

	if err := extractAndWriteCertificates(ctx, kubeClient); err != nil {
		return fmt.Errorf("failed to extract certificates: %w", err)
	}

	logger.V(1).Info("Successfully generated webhook certificates", "certDir", WebhookCertDirectory)

	return nil
}

func checkCertManagerInstalled(ctx context.Context, cfg *rest.Config) (bool, error) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return false, err
	}

	certGVR := schema.GroupVersionResource{
		Group:    certManagerGroup,
		Version:  certManagerVersion,
		Resource: "certificates",
	}

	_, err = dynamicClient.Resource(certGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return false, err
	}

	return true, nil
}

func createSelfSignedIssuer(ctx context.Context, cfg *rest.Config, crClient client.Client) error {
	logger := klog.FromContext(ctx)

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	issuerGVR := schema.GroupVersionResource{
		Group:    certManagerGroup,
		Version:  certManagerVersion,
		Resource: "issuers",
	}

	_, err = dynamicClient.Resource(issuerGVR).Namespace(webhookCertNamespace).Get(ctx, webhookIssuerName, metav1.GetOptions{})
	if err == nil {
		logger.V(2).Info("Issuer already exists")
		return nil
	}

	issuer := map[string]interface{}{
		"apiVersion": fmt.Sprintf("%s/%s", certManagerGroup, certManagerVersion),
		"kind":       issuerKind,
		"metadata": map[string]interface{}{
			"name":      webhookIssuerName,
			"namespace": webhookCertNamespace,
		},
		"spec": map[string]interface{}{
			"selfSigned": map[string]interface{}{},
		},
	}

	unstructuredObj := &unstructured.Unstructured{Object: issuer}
	if err := crClient.Create(ctx, unstructuredObj); err != nil {
		return fmt.Errorf("failed to create issuer: %w", err)
	}

	logger.V(1).Info("Created self-signed issuer")
	return nil
}

func createCertificate(ctx context.Context, cfg *rest.Config, crClient client.Client) error {
	logger := klog.FromContext(ctx)

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	certGVR := schema.GroupVersionResource{
		Group:    certManagerGroup,
		Version:  certManagerVersion,
		Resource: "certificates",
	}

	_, err = dynamicClient.Resource(certGVR).Namespace(webhookCertNamespace).Get(ctx, webhookCertName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	cert := map[string]interface{}{
		"apiVersion": fmt.Sprintf("%s/%s", certManagerGroup, certManagerVersion),
		"kind":       certManagerKind,
		"metadata": map[string]interface{}{
			"name":      webhookCertName,
			"namespace": webhookCertNamespace,
		},
		"spec": map[string]interface{}{
			"secretName": webhookCertSecret,
			"issuerRef": map[string]interface{}{
				"name": webhookIssuerName,
				"kind": issuerKind,
			},
			"commonName": "kube-bind-webhook",
		},
	}

	unstructuredObj := &unstructured.Unstructured{Object: cert}
	if err := crClient.Create(ctx, unstructuredObj); err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	logger.V(1).Info("Created certificate resource")
	return nil
}

func waitForCertificateReady(ctx context.Context, cfg *rest.Config) error {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	certGVR := schema.GroupVersionResource{
		Group:    certManagerGroup,
		Version:  certManagerVersion,
		Resource: "certificates",
	}

	return wait.PollUntilContextTimeout(ctx, 1*time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
		cert, err := dynamicClient.Resource(certGVR).Namespace(webhookCertNamespace).Get(ctx, webhookCertName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		status, found, err := unstructured.NestedMap(cert.Object, "status")
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}

		conditions, found, err := unstructured.NestedSlice(status, "conditions")
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}

		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			if condMap["type"] == "Ready" && condMap["status"] == "True" {
				return true, nil
			}
		}

		return false, nil
	})
}

func extractAndWriteCertificates(ctx context.Context, kubeClient kubernetes.Interface) error {
	secret, err := kubeClient.CoreV1().Secrets(webhookCertNamespace).Get(ctx, webhookCertSecret, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if err := os.MkdirAll(WebhookCertDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	certPath := filepath.Join(WebhookCertDirectory, "tls.crt")
	if certData, exists := secret.Data["tls.crt"]; exists {
		if err := os.WriteFile(certPath, certData, 0600); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
	} else {
		return fmt.Errorf("tls.crt not found in secret")
	}

	keyPath := filepath.Join(WebhookCertDirectory, "tls.key")
	if keyData, exists := secret.Data["tls.key"]; exists {
		if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
			return fmt.Errorf("failed to write key: %w", err)
		}
	} else {
		return fmt.Errorf("tls.key not found in secret")
	}

	return nil
}

func CertsExist(certDir string) bool {
	certFile := filepath.Join(certDir, "tls.crt")
	keyFile := filepath.Join(certDir, "tls.key")

	if _, err := os.Stat(certFile); err != nil {
		return false
	}
	if _, err := os.Stat(keyFile); err != nil {
		return false
	}
	return true
}

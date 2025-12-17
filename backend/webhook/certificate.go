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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	webhookCSRName = "kube-bind-webhook"
)

// GenerateWebhookCertificate generates a certificate for the webhook server using
// Kubernetes Certificate Signing Request API. It creates a CSR, waits for approval,
// and returns a TLS certificate that can be used for the webhook server.
func GenerateWebhookCertificate(ctx context.Context, clientConfig *rest.Config, kubeClient kubernetes.Interface, crClient client.Client, commonName string) (*tls.Certificate, error) {
	logger := klog.FromContext(ctx)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	csrBytes, err := createCertificateRequest(privateKey, commonName)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate request: %w", err)
	}

	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookCSRName,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    csrBytes,
			SignerName: "kubernetes.io/kubelet-serving",
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageServerAuth,
			},
		},
	}

	var existingCSR certificatesv1.CertificateSigningRequest
	err = crClient.Get(ctx, types.NamespacedName{Name: webhookCSRName}, &existingCSR)
	if err == nil {
		if len(existingCSR.Status.Certificate) > 0 {
			logger.V(2).Info("Found existing approved CSR certificate")
			logger.V(2).Info("Creating new CSR as private key cannot be recovered from existing CSR")
			if err := crClient.Delete(ctx, &existingCSR); err != nil {
				logger.V(1).Info("Failed to delete existing CSR, continuing", "error", err)
			}
		} else {
			logger.V(2).Info("Deleting existing unapproved CSR to create new one")
			if err := crClient.Delete(ctx, &existingCSR); err != nil {
				logger.V(1).Info("Failed to delete existing CSR, continuing", "error", err)
			}
		}
	}

	logger.V(1).Info("Creating CertificateSigningRequest for webhook server")
	if err := crClient.Create(ctx, csr); err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	// Auto-approve the CSR if we have permissions
	if err := autoApproveCSR(ctx, kubeClient, webhookCSRName); err != nil {
		logger.V(1).Info("Failed to auto-approve CSR, you may need to approve it manually", "error", err)
		logger.V(1).Info("To approve manually, run: kubectl certificate approve " + webhookCSRName)
	}

	logger.V(1).Info("Waiting for CSR to be approved and certificate to be issued")
	var certBytes []byte
	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (done bool, err error) {
		var csrObj certificatesv1.CertificateSigningRequest
		if err := crClient.Get(ctx, types.NamespacedName{Name: webhookCSRName}, &csrObj); err != nil {
			return false, err
		}

		approved := false
		for _, condition := range csrObj.Status.Conditions {
			if condition.Type == certificatesv1.CertificateApproved && condition.Status == corev1.ConditionTrue {
				approved = true
				break
			}
		}

		if approved && len(csrObj.Status.Certificate) > 0 {
			certBytes = csrObj.Status.Certificate
			return true, nil
		}

		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get certificate from CSR: %w", err)
	}

	return parseCertificate(certBytes, privateKey)
}

func createCertificateRequest(privateKey *rsa.PrivateKey, commonName string) ([]byte, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: commonName,
		},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate request: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	return csrPEM, nil
}

func parseCertificate(certBytes []byte, privateKey *rsa.PrivateKey) (*tls.Certificate, error) {
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}, nil
}

func autoApproveCSR(ctx context.Context, kubeClient kubernetes.Interface, csrName string) error {
	csr, err := kubeClient.CertificatesV1().CertificateSigningRequests().Get(ctx, csrName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved {
			return nil
		}
	}

	approval := certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
		},
		Status: certificatesv1.CertificateSigningRequestStatus{
			Conditions: []certificatesv1.CertificateSigningRequestCondition{
				{
					Type:    certificatesv1.CertificateApproved,
					Status:  corev1.ConditionTrue,
					Reason:  "AutoApproved",
					Message: "Auto-approved by kube-bind backend",
				},
			},
		},
	}

	_, err = kubeClient.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csrName, &approval, metav1.UpdateOptions{})
	return err
}

func GetTLSOptionsForWebhook(ctx context.Context, clientConfig *rest.Config, kubeClient kubernetes.Interface, crClient client.Client) ([]func(*tls.Config), error) {
	commonName := "kube-bind-webhook"

	cert, err := GenerateWebhookCertificate(ctx, clientConfig, kubeClient, crClient, commonName)
	if err != nil {
		klog.FromContext(ctx).V(1).Info("Failed to generate certificate via CSR, will use file-based certs", "error", err)
		return nil, nil
	}

	return []func(*tls.Config){
		func(cfg *tls.Config) {
			cfg.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return cert, nil
			}
		},
	}, nil
}

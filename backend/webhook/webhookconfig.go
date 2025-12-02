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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	validatingWebhookConfigName = "kube-bind-apiserviceexportrequest-validating-webhook"
	WebhookPath                 = "/validate-apiserviceexportrequest"
	WebhookCertDirectory        = "/tmp/k8s-webhook-server/serving-certs"
)

func EnsureValidatingWebhookConfiguration(ctx context.Context, crClient client.Client, kubeClient kubernetes.Interface, webhookURL string, certDir string) error {
	logger := klog.FromContext(ctx)

	_, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("failed to parse webhook URL: %w", err)
	}

	certPath := filepath.Join(certDir, "tls.crt")
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	caCert, err := extractCABundle(certData)
	if err != nil {
		return fmt.Errorf("failed to extract CA bundle: %w", err)
	}

	var existing admissionregistrationv1.ValidatingWebhookConfiguration
	err = crClient.Get(ctx, types.NamespacedName{Name: validatingWebhookConfigName}, &existing)
	if err == nil {
		logger.V(1).Info("Updating existing ValidatingWebhookConfiguration")
		existing.Webhooks = []admissionregistrationv1.ValidatingWebhook{
			buildValidatingWebhook(webhookURL, caCert),
		}
		if err := crClient.Update(ctx, &existing); err != nil {
			return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %w", err)
		}
		logger.V(1).Info("Updated ValidatingWebhookConfiguration")
		return nil
	}

	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: validatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			buildValidatingWebhook(webhookURL, caCert),
		},
	}

	logger.V(1).Info("Creating ValidatingWebhookConfiguration", "name", validatingWebhookConfigName, "url", webhookURL)
	if err := crClient.Create(ctx, webhookConfig); err != nil {
		return fmt.Errorf("failed to create ValidatingWebhookConfiguration: %w", err)
	}

	logger.V(1).Info("Created ValidatingWebhookConfiguration")
	return nil
}

func buildValidatingWebhook(webhookURL string, caCert []byte) admissionregistrationv1.ValidatingWebhook {
	failurePolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	matchPolicy := admissionregistrationv1.Exact

	return admissionregistrationv1.ValidatingWebhook{
		Name:                    "apiserviceexportrequests.kube-bind.io",
		AdmissionReviewVersions: []string{"v1", "v1beta1"},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			URL:      &webhookURL,
			CABundle: caCert,
		},
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"kube-bind.io"},
					APIVersions: []string{"v1alpha2"},
					Resources:   []string{"apiserviceexportrequests", "apiserviceexportrequest"},
				},
			},
		},
		FailurePolicy: &failurePolicy,
		SideEffects:   &sideEffects,
		MatchPolicy:   &matchPolicy,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{},
		},
	}
}

func GetWebhookURL(ctx context.Context, cfg *rest.Config, port int) (string, error) {
	apiServerURL, err := url.Parse(cfg.Host)
	if err != nil {
		return "", fmt.Errorf("failed to parse API server URL: %w", err)
	}

	hostname := apiServerURL.Hostname()

	if hostname == "127.0.0.1" || hostname == "::1" || hostname == "localhost" {
		detectedHostname, err := detectHostnameFromCluster(ctx, cfg)
		if err != nil {
			hostname = "127.0.0.1"
		} else {
			hostname = detectedHostname
		}
	}

	return fmt.Sprintf(
		"https://%s%s",
		net.JoinHostPort(hostname, fmt.Sprintf("%d", port)),
		WebhookPath,
	), nil
}

func detectHostnameFromCluster(ctx context.Context, cfg *rest.Config) (string, error) {
	logger := klog.FromContext(ctx)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no nodes found in cluster")
	}

	node := nodes.Items[0]

	var nodeIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			nodeIP = addr.Address
			break
		}
	}

	if nodeIP == "" {
		return "", fmt.Errorf("node %s has no internal IP", node.Name)
	}

	ip := net.ParseIP(nodeIP)
	if ip == nil {
		return "", fmt.Errorf("invalid node IP: %s", nodeIP)
	}

	gatewayIP := inferGatewayFromNodeIP(ip)
	if gatewayIP == nil {
		return "", fmt.Errorf("could not infer gateway from node IP %s", nodeIP)
	}

	gatewayStr := gatewayIP.String()
	logger.V(1).Info("Inferred gateway IP from node IP", "nodeIP", nodeIP, "gatewayIP", gatewayStr)
	return gatewayStr, nil
}

func inferGatewayFromNodeIP(nodeIP net.IP) net.IP {
	ipv4 := nodeIP.To4()
	if ipv4 == nil {
		return nil
	}

	gateway := make(net.IP, len(ipv4))
	copy(gateway, ipv4)

	lastOctet := gateway[3]
	if lastOctet < 255 {
		gateway[3] = lastOctet + 1
		return gateway
	}

	return nil
}

func extractCABundle(certData []byte) ([]byte, error) {
	var certs []*x509.Certificate
	var block *pem.Block
	rest := certData

	for {
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				continue
			}
			certs = append(certs, cert)
		}
	}

	if len(certs) == 0 {
		return certData, nil
	}

	if len(certs) > 0 && certs[0].IsCA {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certs[0].Raw,
		}), nil
	}

	var caBundle []byte
	for _, cert := range certs {
		caBundle = append(caBundle, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})...)
	}

	if len(certs) == 1 {
		return caBundle, nil
	}

	return caBundle, nil
}

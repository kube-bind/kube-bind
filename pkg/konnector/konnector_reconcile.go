/*
Copyright 2022 The kube bind Authors.

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

package konnector

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster"
)

func (c *controller) reconcile(ctx context.Context, binding *kubebindv1alpha1.ServiceBinding) error {
	logger := klog.FromContext(ctx)

	ref := binding.Spec.KubeconfigSecretRef
	secret, err := c.getSecret(ref.Namespace, ref.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	kubeconfig := secret.StringData[ref.Key]

	c.lock.Lock()
	defer c.lock.Unlock()
	ctrlContext, found := c.controllers[binding.Name]

	// stop existing with old kubeconfig
	if found && ctrlContext.kubeconfig != kubeconfig {
		ctrlContext.serviceBindings.Delete(binding.Name)
		if len(ctrlContext.serviceBindings) == 0 {
			ctrlContext.cancel()
		}
		c.controllers[binding.Name] = nil
	}

	// no need to start a new one
	if kubeconfig == "" {
		return nil
	}

	// find existing with new kubeconfig
	for _, ctrlContext := range c.controllers {
		if ctrlContext.kubeconfig == kubeconfig {
			// add to it
			c.controllers[binding.Name] = ctrlContext
			ctrlContext.serviceBindings.Insert(binding.Name)
			return nil
		}
	}

	// extract which namespace this kubeconfig points to
	var cfg clientcmdapi.Config
	if err := yaml.Unmarshal([]byte(kubeconfig), &cfg); err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The ServiceBinding controller will set a condition
	}
	kubeContext, found := cfg.Contexts[cfg.CurrentContext]
	if !found {
		logger.Error(err, "kubeconfig in secret does not have a current context", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The ServiceBinding controller will set a condition
	}
	if kubeContext.Namespace == "" {
		logger.Error(err, "kubeconfig in secret does not have a namespace set for the current context", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The ServiceBinding controller will set a condition
	}
	providerNamespace := kubeContext.Namespace
	providerConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The ServiceBinding controller will set a condition
	}

	// create new because there is none yet for this kubeconfig
	ctrl, err := cluster.NewController(
		binding.Spec.KubeconfigSecretRef.Namespace+"/"+binding.Spec.KubeconfigSecretRef.Name,
		providerNamespace,
		c.consumerConfig,
		providerConfig,
		c.namespaceInformer,
		c.namespaceLister,
		c.serviceBindingInformer,
		c.serviceBindingLister,
	)
	if err != nil {
		logger.Error(err, "failed to start new cluster controller")
		return err
	}

	ctrlCtx, cancel := context.WithCancel(ctx)
	c.controllers[binding.Name] = &controllerContext{
		kubeconfig:      kubeconfig,
		cancel:          cancel,
		serviceBindings: sets.NewString(binding.Name),
	}
	go ctrl.Start(ctrlCtx)

	return nil
}

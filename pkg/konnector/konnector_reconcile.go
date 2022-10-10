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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type startable interface {
	Start(ctx context.Context)
}

type reconciler struct {
	lock        sync.Mutex
	controllers map[string]*controllerContext // by service binding name

	newClusterController func(consumerSecretRefKey, providerNamespace string, providerConfig *rest.Config) (startable, error)
	getSecret            func(ns, name string) (*corev1.Secret, error)
}

type controllerContext struct {
	kubeconfig      string
	cancel          func()
	serviceBindings sets.String // when this is empty, the controller should be stopped by closing the context
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha1.ServiceBinding) error {
	logger := klog.FromContext(ctx)

	var kubeconfig string

	ref := binding.Spec.KubeconfigSecretRef
	secret, err := r.getSecret(ref.Namespace, ref.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("secret not found", "secret", ref.Namespace+"/"+ref.Name)
	} else {
		kubeconfig = string(secret.Data[ref.Key])
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	ctrlContext, found := r.controllers[binding.Name]

	// stop existing with old kubeconfig
	if found && ctrlContext.kubeconfig != kubeconfig {
		logger.V(2).Info("stopping controller with old kubeconfig", "secret", ref.Namespace+"/"+ref.Name)
		ctrlContext.serviceBindings.Delete(binding.Name)
		if len(ctrlContext.serviceBindings) == 0 {
			ctrlContext.cancel()
		}
		r.controllers[binding.Name] = nil
	}

	// no need to start a new one
	if kubeconfig == "" {
		return nil
	}

	// find existing with new kubeconfig
	for _, ctrlContext := range r.controllers {
		if ctrlContext.kubeconfig == kubeconfig {
			// add to it
			logger.V(2).Info("adding to existing controller", "secret", ref.Namespace+"/"+ref.Name)
			r.controllers[binding.Name] = ctrlContext
			ctrlContext.serviceBindings.Insert(binding.Name)
			return nil
		}
	}

	// extract which namespace this kubeconfig points to
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
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
	logger.V(2).Info("starting new controller", "secret", ref.Namespace+"/"+ref.Name)
	ctrl, err := r.newClusterController(
		binding.Spec.KubeconfigSecretRef.Namespace+"/"+binding.Spec.KubeconfigSecretRef.Name,
		providerNamespace,
		providerConfig,
	)
	if err != nil {
		logger.Error(err, "failed to start new cluster controller")
		return err
	}

	ctrlCtx, cancel := context.WithCancel(ctx)
	r.controllers[binding.Name] = &controllerContext{
		kubeconfig:      kubeconfig,
		cancel:          cancel,
		serviceBindings: sets.NewString(binding.Name),
	}
	go ctrl.Start(ctrlCtx)

	return nil
}

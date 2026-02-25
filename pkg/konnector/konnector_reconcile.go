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

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type startable interface {
	Start(ctx context.Context)
}

// reconciler manages cluster controllers for provider connections.
//
// Controller Sharing Model:
// -------------------------
// Multiple APIServiceBindings can share the same cluster controller if they use the same
// kubeconfig (i.e., connect to the same provider namespace). This is tracked via controllerContext:
//
//	r.controllers map:
//	  "__heartbeat__kubeconfig-xyz" -> controllerContext{kubeconfig: "...", serviceBindings: ["__heartbeat__kubeconfig-xyz", "binding-a", "binding-b"]}
//	  "binding-a"                   -> (same controllerContext as above)
//	  "binding-b"                   -> (same controllerContext as above)
//
// Flow when APIServiceBinding arrives using same kubeconfig as existing heartbeat controller:
//  1. reconcile() is called with the new binding
//  2. It extracts kubeconfig from binding.Spec.KubeconfigSecretRef
//  3. It loops through r.controllers looking for matching kubeconfig (lines 174-182)
//  4. Finds the existing controllerContext (created by startClusterControllerForSecret)
//  5. Points r.controllers[binding.Name] to the SAME controllerContext
//  6. Adds binding.Name to controllerContext.serviceBindings set
//  7. No new controller is started - the existing one handles everything
type reconciler struct {
	lock        sync.RWMutex
	controllers map[string]*controllerContext // keyed by binding name or synthetic heartbeat key

	newClusterController func(consumerSecretRefKey, providerNamespace string, reconcileServiceBinding func(binding *kubebindv1alpha2.APIServiceBinding) bool, providerConfig *rest.Config) (startable, error)
	getSecret            func(ns, name string) (*corev1.Secret, error)
}

// controllerContext tracks a running cluster controller and the bindings it serves.
// Multiple map entries in reconciler.controllers can point to the same controllerContext
// when they share the same kubeconfig.
type controllerContext struct {
	kubeconfig      string           // the kubeconfig content - used to match bindings to controllers
	cancel          func()           // cancels the controller's context when no bindings remain
	serviceBindings sets.Set[string] // all binding names (including synthetic heartbeat keys) using this controller
}

// startClusterControllerForSecret starts a cluster controller for the given kubeconfig secret.
// This enables heartbeat reporting immediately without requiring an APIServiceBinding.
//
// The controller is registered with a synthetic key "__heartbeat__<secret-name>".
// When an APIServiceBinding later arrives using the same kubeconfig, reconcile() will:
// 1. Find this existing controller by matching kubeconfig content
// 2. Add the binding to the same controllerContext.serviceBindings set
// 3. Point r.controllers[binding.Name] to the same controllerContext
// 4. NOT start a new controller (reuses existing one)
func (r *reconciler) startClusterControllerForSecret(ctx context.Context, secret *corev1.Secret) error {
	logger := klog.FromContext(ctx)

	kubeconfig := string(secret.Data["kubeconfig"])
	if kubeconfig == "" {
		logger.V(2).Info("secret does not contain kubeconfig", "secret", secret.Namespace+"/"+secret.Name)
		return nil
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	// Check if we already have a controller for this kubeconfig
	for _, ctrlContext := range r.controllers {
		if ctrlContext.kubeconfig == kubeconfig {
			logger.V(2).Info("cluster controller already exists for secret", "secret", secret.Namespace+"/"+secret.Name)
			return nil
		}
	}

	// Extract which namespace this kubeconfig points to
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", secret.Namespace, "name", secret.Name)
		return nil // nothing we can do here
	}
	kubeContext, found := cfg.Contexts[cfg.CurrentContext]
	if !found {
		logger.Error(err, "kubeconfig in secret does not have a current context", "namespace", secret.Namespace, "name", secret.Name)
		return nil
	}
	if kubeContext.Namespace == "" {
		logger.Error(err, "kubeconfig in secret does not have a namespace set for the current context", "namespace", secret.Namespace, "name", secret.Name)
		return nil
	}
	providerNamespace := kubeContext.Namespace
	providerConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", secret.Namespace, "name", secret.Name)
		return nil
	}

	// Use the secret name as a synthetic binding key for tracking.
	// This key is used to track the controller context, but the controller
	// will also handle real APIServiceBindings that use the same kubeconfig.
	syntheticKey := "__heartbeat__" + secret.Name

	ctrlCtx, cancel := context.WithCancel(ctx)
	ctrlContext := &controllerContext{
		kubeconfig:      kubeconfig,
		cancel:          cancel,
		serviceBindings: sets.New(syntheticKey),
	}
	r.controllers[syntheticKey] = ctrlContext

	// Create and start the cluster controller.
	// The reconcileServiceBinding function checks if a binding should be processed
	// by this controller. We check if the binding is in our serviceBindings set,
	// which will include both the synthetic heartbeat key and any real bindings
	// that get added later when they use the same kubeconfig.
	logger.Info("starting cluster controller for early heartbeat", "secret", secret.Namespace+"/"+secret.Name, "providerNamespace", providerNamespace)
	ctrl, err := r.newClusterController(
		secret.Namespace+"/"+secret.Name,
		providerNamespace,
		func(svcBinding *kubebindv1alpha2.APIServiceBinding) bool {
			r.lock.RLock()
			defer r.lock.RUnlock()
			// Check if this binding is registered with this controller context
			return ctrlContext.serviceBindings.Has(svcBinding.Name)
		},
		providerConfig,
	)
	if err != nil {
		logger.Error(err, "failed to start cluster controller for heartbeat")
		cancel()
		delete(r.controllers, syntheticKey)
		return err
	}

	go ctrl.Start(ctrlCtx)

	return nil
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding) error {
	logger := klog.FromContext(ctx)

	var kubeconfig string

	ref := binding.Spec.KubeconfigSecretRef
	secret, err := r.getSecret(ref.Namespace, ref.Name)
	switch {
	case errors.IsNotFound(err):
		logger.V(2).Info("secret not found", "secret", ref.Namespace+"/"+ref.Name)
	case err != nil:
		return err
	default:
		kubeconfig = string(secret.Data[ref.Key])
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	ctrlContext, found := r.controllers[binding.Name]

	// stop existing with old kubeconfig
	if found && ctrlContext.kubeconfig != kubeconfig {
		logger.V(2).Info("stopping Controller with old kubeconfig", "secret", ref.Namespace+"/"+ref.Name)
		ctrlContext.serviceBindings.Delete(binding.Name)
		if len(ctrlContext.serviceBindings) == 0 {
			ctrlContext.cancel()
		}
		delete(r.controllers, binding.Name)
	}

	// no need to start a new one
	if kubeconfig == "" {
		return nil
	}

	// Find existing controller with the same kubeconfig.
	// This handles the case where:
	// 1. A heartbeat controller was started from a labeled secret (via startClusterControllerForSecret)
	// 2. An APIServiceBinding now arrives that uses the same kubeconfig
	// 3. Instead of starting a duplicate controller, we reuse the existing one
	//
	// The binding is added to the existing controllerContext's serviceBindings set,
	// and r.controllers[binding.Name] points to the same controllerContext.
	for _, ctrlContext := range r.controllers {
		if ctrlContext.kubeconfig == kubeconfig {
			// Reuse existing controller - no new controller started
			logger.V(2).Info("adding binding to existing Controller", "secret", ref.Namespace+"/"+ref.Name)
			r.controllers[binding.Name] = ctrlContext
			ctrlContext.serviceBindings.Insert(binding.Name)
			return nil
		}
	}

	// extract which namespace this kubeconfig points to
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The APIServiceBinding Controller will set a condition
	}
	kubeContext, found := cfg.Contexts[cfg.CurrentContext]
	if !found {
		logger.Error(err, "kubeconfig in secret does not have a current context", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The APIServiceBinding Controller will set a condition
	}
	if kubeContext.Namespace == "" {
		logger.Error(err, "kubeconfig in secret does not have a namespace set for the current context", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The APIServiceBinding Controller will set a condition
	}
	providerNamespace := kubeContext.Namespace
	providerConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		logger.Error(err, "invalid kubeconfig in secret", "namespace", ref.Namespace, "name", ref.Name)
		return nil // nothing we can do here. The APIServiceBinding Controller will set a condition
	}

	ctrlCtx, cancel := context.WithCancel(ctx)
	r.controllers[binding.Name] = &controllerContext{
		kubeconfig:      kubeconfig,
		cancel:          cancel,
		serviceBindings: sets.New(binding.Name),
	}

	// create new because there is none yet for this kubeconfig
	logger.V(2).Info("starting new Controller", "secret", ref.Namespace+"/"+ref.Name)
	ctrl, err := r.newClusterController(
		binding.Spec.KubeconfigSecretRef.Namespace+"/"+binding.Spec.KubeconfigSecretRef.Name,
		providerNamespace,
		func(svcBinding *kubebindv1alpha2.APIServiceBinding) bool {
			r.lock.RLock()
			defer r.lock.RUnlock()
			return r.controllers[binding.Name].serviceBindings.Has(svcBinding.Name)
		},
		providerConfig,
	)
	if err != nil {
		logger.Error(err, "failed to start new cluster Controller")
		return err
	}

	go ctrl.Start(ctrlCtx)

	return nil
}

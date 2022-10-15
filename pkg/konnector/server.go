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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/deploy/crd"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type Server struct {
	Config     *Config
	Controller *Controller
}

func NewServer(config *Config) (*Server, error) {
	// construct controllers
	k, err := New(
		config.ClientConfig,
		config.BindInformers.KubeBind().V1alpha1().APIServiceBindings(),
		config.KubeInformers.Core().V1().Secrets(), // TODO(sttts): watch individual secrets for security and memory consumption
		config.KubeInformers.Core().V1().Namespaces(),
		config.ApiextensionsInformers.Apiextensions().V1().CustomResourceDefinitions(),
	)
	if err != nil {
		return nil, err
	}

	s := &Server{
		Config:     config,
		Controller: k,
	}

	return s, nil
}

func (s *Server) OptionallyStartInformers(ctx context.Context) {
	logger := klog.FromContext(ctx)

	// start informer factories
	logger.Info("starting informers")
	s.Config.KubeInformers.Start(ctx.Done())
	s.Config.BindInformers.Start(ctx.Done())
	s.Config.ApiextensionsInformers.Start(ctx.Done())
	kubeSynced := s.Config.KubeInformers.WaitForCacheSync(ctx.Done())
	kubeBindSynced := s.Config.BindInformers.WaitForCacheSync(ctx.Done())
	apiextensionsSynced := s.Config.ApiextensionsInformers.WaitForCacheSync(ctx.Done())

	logger.Info("local informers are synced",
		"kubeSynced", fmt.Sprintf("%v", kubeSynced),
		"kubeBindSynced", fmt.Sprintf("%v", kubeBindSynced),
		"apiextensionsSynced", fmt.Sprintf("%v", apiextensionsSynced),
	)
}

func (s *Server) Run(ctx context.Context) error {
	// install/upgrade CRDs
	if err := crd.Create(ctx,
		s.Config.ApiextensionsClient.ApiextensionsV1().CustomResourceDefinitions(),
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiservicebindings"},
	); err != nil {
		return err
	}

	s.Controller.Start(ctx, 2)
	return nil
}

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

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	backend "github.com/kube-bind/kube-bind/contrib/example-backend"
	"github.com/kube-bind/kube-bind/contrib/example-backend/options"
)

func main() {
	err := run(genericapiserver.SetupSignalContext())
	klog.Flush()

	if err != nil {
		fmt.Printf("Error running example backend: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	options := options.NewOptions()
	options.AddFlags(pflag.CommandLine)
	pflag.Parse()

	// setup logging first
	if err := logsv1.ValidateAndApply(options.Logs, nil); err != nil {
		return err
	}

	// Set up controller-runtime logger early to avoid warnings
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.SetLogger(klog.NewKlogr())

	ver := version.Get().GitVersion
	if i := strings.Index(ver, "bind-"); i != -1 {
		ver = ver[i+5:] // example: v1.25.2+kubectl-bind-v0.0.7-52-g8fee0baeaff3aa
	}
	logger := klog.FromContext(ctx)
	logger.Info("Starting example-backend", "version", ver)

	// create server
	completed, err := options.Complete()
	if err != nil {
		return err
	}
	if err := completed.Validate(); err != nil {
		return err
	}

	// start server
	config, err := backend.NewConfig(completed)
	if err != nil {
		return err
	}
	server, err := backend.NewServer(ctx, config)
	if err != nil {
		return err
	}

	if err := server.Run(ctx); err != nil {
		return err
	}
	logger.Info("Listening", "address", server.Addr())

	<-ctx.Done()

	return nil
}

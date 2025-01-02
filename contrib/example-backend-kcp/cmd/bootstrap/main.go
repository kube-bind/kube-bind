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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/pflag"

	genericapiserver "k8s.io/apiserver/pkg/server"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"

	bootstrap "github.com/kube-bind/kube-bind/contrib/example-backend-kcp/bootstrap"
	"github.com/kube-bind/kube-bind/contrib/example-backend-kcp/bootstrap/options"
)

func main() {
	ctx := genericapiserver.SetupSignalContext()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err) // nolint: errcheck
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	defer klog.Flush()

	options := options.NewOptions()
	options.AddFlags(pflag.CommandLine)
	pflag.Parse()

	logger := klog.FromContext(ctx)
	logger.Info("bootstrapping api")

	// setup logging first
	if err := logsv1.ValidateAndApply(options.Logs, nil); err != nil {
		return err
	}

	// create init server
	completed, err := options.Complete()
	if err != nil {
		return err
	}
	if err := completed.Validate(); err != nil {
		return err
	}

	// start server
	config, err := bootstrap.NewConfig(completed)
	if err != nil {
		return err
	}

	server, err := bootstrap.NewServer(ctx, config)
	if err != nil {
		return err
	}

	return server.Start(ctx)
}

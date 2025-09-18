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

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/leaderelection"
	logsv1 "k8s.io/component-base/logs/api/v1"
	componentbaseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector"
	konnectoroptions "github.com/kube-bind/kube-bind/pkg/konnector/options"
	bindversion "github.com/kube-bind/kube-bind/pkg/version"

	_ "k8s.io/component-base/logs/json/register"
)

const LeaderElectionTimeout = 20 * time.Second

func New(ctx context.Context) *cobra.Command {
	ver, err := bindversion.BinaryVersion(componentbaseversion.Get().GitVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get version: %v\n", err)
		ver = "<unknown>"
	}

	options := konnectoroptions.NewOptions()
	cmd := &cobra.Command{
		Use:     "konnector",
		Short:   "Connect remote API services to local APIs",
		Version: ver,
		RunE: func(cmd *cobra.Command, args []string) error {
			// setup logging first
			if err := logsv1.ValidateAndApply(options.Logs, nil); err != nil {
				return err
			}

			logger := klog.FromContext(ctx)
			logger.Info("Starting konnector", "version", ver)

			// setup server
			completed, err := options.Complete()
			if err != nil {
				return err
			}
			if err := completed.Validate(); err != nil {
				return err
			}
			config, err := konnector.NewConfig(completed)
			if err != nil {
				return err
			}
			server, err := konnector.NewServer(config)
			if err != nil {
				return err
			}
			prepared, err := server.PrepareRun(ctx)
			if err != nil {
				return err
			}
			prepared.OptionallyStartInformers(ctx)

			logger.Info("trying to acquire the lock")
			lock := NewLock(config.KubeClient, options.LeaseLockNamespace, options.LeaseLockName, options.LeaseLockIdentity)

			le := makeLeaderElectorOrDie(ctx, lock, options.LeaseLockIdentity, func(ctx context.Context) {
				logger.Info("starting konnector controller")
				err = prepared.Run(ctx)
			})

			hz := leaderelection.NewLeaderHealthzAdaptor(LeaderElectionTimeout)
			hz.SetLeaderElection(le)

			server.AddCheck(hz)
			server.StartHealthCheck(ctx)

			le.Run(ctx)
			<-ctx.Done()

			return err
		},
	}
	options.AddFlags(cmd.Flags())

	return cmd
}

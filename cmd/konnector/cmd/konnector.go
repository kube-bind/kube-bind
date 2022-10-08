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

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version"

	konnectoroptions "github.com/kube-bind/kube-bind/cmd/konnector/options"
)

func New() *cobra.Command {
	options := konnectoroptions.NewOptions()
	cmd := &cobra.Command{
		Use:   "konnector",
		Short: "Connect remote API services to local APIs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logsv1.ValidateAndApply(options.Logs, nil); err != nil {
				return err
			}
			if err := options.Complete(); err != nil {
				return err
			}

			if err := options.Validate(); err != nil {
				return err
			}

			ctx := genericapiserver.SetupSignalContext()
			if err := Run(options, ctx); err != nil {
				return err
			}

			<-ctx.Done()

			return nil
		},
	}

	options.AddFlags(cmd.Flags())

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	return cmd
}

func Run(options *konnectoroptions.Options, ctx context.Context) error {
	_, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{
		ExplicitPath: options.KubeConfigPath,
	}, nil).ClientConfig()
	if err != nil {
		return err
	}

	/*
		if err := syncer.StartSyncer(
			ctx,
			&syncer.SyncerConfig{
				UpstreamConfig:      upstreamConfig,
				DownstreamConfig:    downstreamConfig,
				ResourcesToSync:     sets.NewString(options.SyncedResourceTypes...),
				SyncTargetWorkspace: logicalcluster.New(options.FromClusterName),
				SyncTargetName:      options.SyncTargetName,
				SyncTargetUID:       options.SyncTargetUID,
			},
			numThreads,
			options.APIImportPollInterval,
		); err != nil {
			return err
		}*/

	return nil
}

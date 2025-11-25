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

package plugin

import (
	"fmt"
	"os"
	"runtime"

	"sigs.k8s.io/kind/pkg/cluster"
)

func (o *DevOptions) RunDelete() error {
	if err := o.deleteCluster(o.ProviderClusterName); err != nil {
		return err
	}

	if err := o.deleteCluster(o.ConsumerClusterName); err != nil {
		return err
	}

	return o.cleanupHostEntries()
}

func (o *DevOptions) deleteCluster(clusterName string) error {
	fmt.Fprintf(o.Streams.ErrOut, "Deleting kind cluster %s\n", clusterName)
	provider := cluster.NewProvider()

	err := provider.Delete(clusterName, "")
	if err != nil {
		return err
	}

	kubeconfigPath := fmt.Sprintf("%s.kubeconfig", clusterName)
	if err := os.Remove(kubeconfigPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(o.Streams.ErrOut, "Failed to remove kubeconfig file %s: %v\n", kubeconfigPath, err)
	} else {
		fmt.Fprintf(o.Streams.ErrOut, "Removed kubeconfig file %s\n", kubeconfigPath)
	}

	return nil
}

func (o *DevOptions) cleanupHostEntries() error {
	if err := removeHostEntry("kube-bind.dev.local"); err != nil {
		fmt.Fprintf(o.Streams.ErrOut, "Failed to remove host entry: %v\n", err)
		fmt.Fprintf(o.Streams.ErrOut, "Warning: Could not automatically remove host entry. Please run:\n")
		if runtime.GOOS == "windows" {
			fmt.Fprintf(o.Streams.ErrOut, "  Remove '127.0.0.1 kube-bind.dev.local' line from C:\\Windows\\System32\\drivers\\etc\\hosts\n")
		} else {
			fmt.Fprintf(o.Streams.ErrOut, "  sudo sed -i '/127.0.0.1 kube-bind.dev.local/d' /etc/hosts\n")
		}
	} else {
		fmt.Fprintf(o.Streams.ErrOut, "Removed host entry kube-bind.dev.local\n")
	}
	return nil
}

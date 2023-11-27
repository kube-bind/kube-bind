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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func NewLock(client kubeclient.Interface, namespace, lockName, podName string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockName,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podName,
		},
	}
}

func makeLeaderElectorOrDie(ctx context.Context, lock *resourcelock.LeaseLock, id string, run func(ctx context.Context)) *leaderelection.LeaderElector {
	logger := klog.FromContext(ctx)

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				logger.Info("started leading", "id", id)
				run(ctx)
			},
			OnStoppedLeading: func() {
				logger.Info("no longer the leader, staying inactive.")
			},
			OnNewLeader: func(currentID string) {
				if currentID == id {
					logger.Info("still the leader!")
					return
				}
				logger.Info("new leader", "id", currentID)
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return le
}

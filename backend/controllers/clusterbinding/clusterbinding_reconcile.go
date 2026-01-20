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

package clusterbinding

import (
	"context"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct{}

func (r *reconciler) reconcile(ctx context.Context, client client.Client, cache cache.Cache, clusterBinding *kubebindv1alpha2.ClusterBinding) error {
	var errs []error

	if err := r.ensureClusterBindingConditions(ctx, clusterBinding); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(clusterBinding)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureClusterBindingConditions(_ context.Context, clusterBinding *kubebindv1alpha2.ClusterBinding) error {
	if clusterBinding.Status.LastHeartbeatTime.IsZero() {
		conditions.MarkFalse(clusterBinding,
			kubebindv1alpha2.ClusterBindingConditionHealthy,
			"FirstHeartbeatPending",
			conditionsapi.ConditionSeverityInfo,
			"Waiting for first heartbeat",
		)
	} else if clusterBinding.Status.HeartbeatInterval.Duration == 0 {
		conditions.MarkFalse(clusterBinding,
			kubebindv1alpha2.ClusterBindingConditionHealthy,
			"HeartbeatIntervalMissing",
			conditionsapi.ConditionSeverityInfo,
			"Waiting for consumer cluster reporting its heartbeat interval",
		)
	} else if ago := time.Since(clusterBinding.Status.LastHeartbeatTime.Time); ago > clusterBinding.Status.HeartbeatInterval.Duration*2 {
		conditions.MarkFalse(clusterBinding,
			kubebindv1alpha2.ClusterBindingConditionHealthy,
			"HeartbeatTimeout",
			conditionsapi.ConditionSeverityError,
			"Heartbeat timeout: expected heartbeat within %s, but last one has been at %s",
			clusterBinding.Status.HeartbeatInterval.Duration,
			clusterBinding.Status.LastHeartbeatTime.Time, // do not put "ago" here. It will hotloop.
		)
	} else if ago < time.Second*10 {
		conditions.MarkFalse(clusterBinding,
			kubebindv1alpha2.ClusterBindingConditionHealthy,
			"HeartbeatTimeDrift",
			conditionsapi.ConditionSeverityWarning,
			"Clocks of consumer cluster and service account cluster seem to be off by more than 10s",
		)
	} else {
		conditions.MarkTrue(clusterBinding,
			kubebindv1alpha2.ClusterBindingConditionHealthy,
		)
	}

	return nil
}

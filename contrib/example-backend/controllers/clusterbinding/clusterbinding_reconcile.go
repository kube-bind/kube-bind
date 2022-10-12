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

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct{}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha1.ClusterBinding) error {
	if export.Status.LastHeartbeatTime.IsZero() {
		conditions.MarkFalse(export,
			kubebindv1alpha1.ClusterBindingConditionHealthy,
			"FirstHeartbeatPending",
			conditionsapi.ConditionSeverityInfo,
			"Waiting for first heartbeat",
		)
	} else if export.Status.HeartbeatInterval.Duration == 0 {
		conditions.MarkFalse(export,
			kubebindv1alpha1.ClusterBindingConditionHealthy,
			"HeartbeatIntervalMissing",
			conditionsapi.ConditionSeverityInfo,
			"Waiting for consumer cluster reporting its heartbeat interval",
		)
	} else if ago := time.Since(export.Status.LastHeartbeatTime.Time); ago > export.Status.HeartbeatInterval.Duration*2 {
		conditions.MarkFalse(export,
			kubebindv1alpha1.ClusterBindingConditionHealthy,
			"HeartbeatTimeout",
			conditionsapi.ConditionSeverityError,
			"Heartbeat timeout: expected heartbeat within %s, but last one has been %s ago",
			export.Status.HeartbeatInterval.Duration,
			ago,
		)
	} else if ago < time.Second*10 {
		conditions.MarkFalse(export,
			kubebindv1alpha1.ClusterBindingConditionHealthy,
			"HeartbeatTimeDrift",
			conditionsapi.ConditionSeverityWarning,
			"Clocks of consumer cluster and service account cluster seem to be off by more than 10s",
		)
	} else {
		conditions.MarkTrue(export,
			kubebindv1alpha1.ClusterBindingConditionHealthy,
		)
	}

	return nil
}

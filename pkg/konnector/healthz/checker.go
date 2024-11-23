/*
Copyright 2023 The Kube Bind Authors.

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

package healthz

import (
	"encoding/json"
	"net/http"

	"k8s.io/apiserver/pkg/server/healthz"
)

type HealthChecker = healthz.HealthChecker

type checker struct {
	checks []healthz.HealthChecker
}

func (c checker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{}
	allOk := true

	for _, c := range c.checks {
		err := c.Check(r)
		if err != nil {
			allOk = false
			status[c.Name()] = "degraded"
			continue
		}
		status[c.Name()] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	if allOk {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	json.NewEncoder(w).Encode(status) //nolint:errcheck
}

func (c *checker) AddCheck(check healthz.HealthChecker) {
	c.checks = append(c.checks, check)
}

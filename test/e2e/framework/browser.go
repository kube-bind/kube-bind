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

package framework

import (
	"testing"
	"time"

	"github.com/headzoo/surf/browser"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/util/wait"
)

func BrowerEventuallyAtPath(t *testing.T, browser *browser.Browser, path string) {
	require.Eventuallyf(t, func() bool {
		if browser.Url().Path == path {
			t.Logf("Browser is at %s, waiting for path %s", browser.Url(), path)
			return true
		}
		t.Logf("Waiting for browser to be at %s, current URL: %s", path, browser.Url())
		return false
	}, wait.ForeverTestTimeout, time.Millisecond*100, "Browser is not at path %s", path)
}

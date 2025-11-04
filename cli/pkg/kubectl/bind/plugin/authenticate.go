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

package plugin

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

func validateProviderVersion(providerVersion string) error {
	switch providerVersion {
	case "":
		return fmt.Errorf("provider version %q is empty, please update the backend to v0.5.0+", providerVersion)
	case "v0.0.0", "v0.0.0-master+$Format:%H$":
		// unversioned, development version
		return nil
	}

	providerSemVer, err := semver.Parse(strings.TrimPrefix(providerVersion, "v"))
	if err != nil {
		return fmt.Errorf("provider version %q cannot be parsed", providerVersion)
	}
	// Check if provider is higher than 0.4.8, we need to have same version of kube-bind to use this provider.
	if min := semver.MustParse("0.6.0"); providerSemVer.LT(min) {
		return fmt.Errorf("provider version %s is not supported, must be at least v%s", providerVersion, min)
	}

	return nil
}

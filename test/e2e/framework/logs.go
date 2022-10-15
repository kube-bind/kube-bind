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
	"os"

	"github.com/spf13/pflag"

	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
)

func init() {
	logs := logs.NewOptions()
	logs.Verbosity = logsv1.VerbosityLevel(2)
	logsv1.AddFlags(logs, pflag.CommandLine)
	if err := pflag.CommandLine.Parse(os.Args); err != nil {
		panic(err)
	}
	if err := logsv1.ValidateAndApply(logs, nil); err != nil {
		panic(err)
	}
}

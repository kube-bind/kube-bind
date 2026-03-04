/*
Copyright 2026 The Kube Bind Authors.

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

package redis

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	Address  string
	Password string
}

func NewOptions() *Options {
	return &Options{}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.Address, "redis-addr", options.Address, "The redis address (e.g. localhost:6379) to connect to if storing sessions in Redis. If empty, the in-memory provider is used.")
	fs.StringVar(&options.Password, "redis-password", options.Password, "The connection password to use if storing sessions in Redis.")
}
func (options *Options) Validate() error {
	if options.Password != "" && options.Address == "" {
		return fmt.Errorf("redis-addr must be specified when using redis-password")
	}
	return nil
}

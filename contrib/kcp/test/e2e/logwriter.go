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

package e2e

import "testing"

type logWriter struct {
	prefix string
	tlog   func(args ...any)
}

func newLogWriter(prefix string, t testing.TB) *logWriter {
	return &logWriter{
		prefix: prefix,
		tlog:   t.Log,
	}
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	lw.tlog(lw.prefix + string(p))
	return len(p), nil
}

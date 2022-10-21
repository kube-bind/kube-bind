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

package version

import (
	"testing"
)

func TestBinaryVersion(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{
			name: "happy case",
			arg:  "v1.25.2+kube-bind-v0.0.15",
			want: "v0.0.15",
		},
		{
			name: "dirty",
			arg:  "v1.25.2+kube-bind-v0.0.15",
			want: "v0.0.15",
		},
		{
			name:    "wrong prefix",
			arg:     "v1.25.2+kube-bla-v0.0.15",
			wantErr: true,
		},
		{
			name:    "garbage",
			arg:     "v1.25.2+foo",
			wantErr: true,
		},
		{
			name: "no ldflags",
			arg:  "v0.0.0-master+$Format:%H$",
			want: "v0.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BinaryVersion(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("binaryVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("binaryVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

#!/usr/bin/env bash

# Copyright 2022 The Kube Bind Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e
set -o pipefail

VERSION=$(grep "go 1." go.mod | sed 's/go //' | sed 's/.0$//')

grep "FROM golang:" Dockerfile | { ! grep -v "${VERSION}"; } || { echo "Wrong go version in Dockerfile, expected ${VERSION}"; exit 1; }
grep "go-version:" .github/workflows/*.yaml | { ! grep -v "go-version: v${VERSION}"; } || { echo "Wrong go version in .github/workflows/*.yaml, expected ${VERSION}"; exit 1; }
grep "golang:" .ko.yaml | { ! grep -v "golang:${VERSION}"; } || { echo "Wrong go version in .ko.yaml, expected ${VERSION}"; exit 1; }

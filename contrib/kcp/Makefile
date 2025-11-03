# Copyright 2025 The Kube Bind Authors.
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

# We need bash for some conditional logic below.
SHELL := /usr/bin/env bash -e

ROOT_DIR=$(abspath ../..)
UGET = $(ROOT_DIR)/hack/uget.sh
ABS_TOOLS_DIR = $(ROOT_DIR)/hack/tools
ABS_BUILD_DIR = $(ROOT_DIR)/bin
PATH := $(ABS_BUILD_DIR):$(ABS_TOOLS_DIR):$(PATH)

export UGET_DIRECTORY = $(ABS_TOOLS_DIR)
export UGET_CHECKSUMS = $(ROOT_DIR)/hack/tools.checksums

KCP_APIGEN_VER := v0.28.0
KCP_APIGEN_BIN := apigen
KCP_APIGEN_GEN := $(ABS_TOOLS_DIR)/$(KCP_APIGEN_BIN)
export KCP_APIGEN_GEN # so hack scripts can use it

all: build
.PHONY: all

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

# KCP tool installation targets
$(KCP_APIGEN_GEN):
	@GO_MODULE=true $(UGET) github.com/kcp-dev/kcp/sdk/cmd/apigen $(KCP_APIGEN_BIN) $(KCP_APIGEN_VER)

# KCP runtime targets
run-kcp: $(KCP) ## Start KCP server
	$(KCP) start -v=8

# Generate KCP API resources
codegen: $(KCP_APIGEN_GEN) ## Generate KCP API resources from CRDs
	./hack/update-kcp-codegen.sh

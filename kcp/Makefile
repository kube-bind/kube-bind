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

GO_INSTALL = ../hack/go-install.sh

ROOT_DIR=$(abspath ..)
TOOLS_DIR=../hack/tools
TOOLS_GOBIN_DIR := $(abspath $(TOOLS_DIR))
GOBIN_DIR=$(abspath ../bin )
PATH := $(GOBIN_DIR):$(TOOLS_GOBIN_DIR):$(PATH)
TMPDIR := $(shell mktemp -d)

# Detect the path used for the install target
ifeq (,$(shell go env GOBIN))
INSTALL_GOBIN=$(shell go env GOPATH)/bin
else
INSTALL_GOBIN=$(shell go env GOBIN)
endif


KCP_APIGEN_VER := v0.28.0
KCP_APIGEN_BIN := apigen
KCP_APIGEN_GEN := $(TOOLS_GOBIN_DIR)/$(KCP_APIGEN_BIN)-$(KCP_APIGEN_VER)
export KCP_APIGEN_GEN # so hack scripts can use it

all: build
.PHONY: all

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

# KCP tool installation targets
$(KCP_APIGEN_GEN):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/kcp-dev/kcp/sdk/cmd/apigen $(KCP_APIGEN_BIN) $(KCP_APIGEN_VER)


# KCP runtime targets
run-kcp: $(KCP) ## Start KCP server
	$(KCP) start -v=8

# Generate KCP API resources
codegen: $(KCP_APIGEN_GEN) ## Generate KCP API resources from CRDs
	./hack/update-kcp-codegen.sh

# Copyright 2021 The Kubectl Bind contributors.
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

GO_INSTALL = ./hack/go-install.sh

TOOLS_DIR=hack/tools
TOOLS_GOBIN_DIR := $(abspath $(TOOLS_DIR))
GOBIN_DIR=$(abspath ./bin )
PATH := $(GOBIN_DIR):$(TOOLS_GOBIN_DIR):$(PATH)
TMPDIR := $(shell mktemp -d)

# Detect the path used for the install target
ifeq (,$(shell go env GOBIN))
INSTALL_GOBIN=$(shell go env GOPATH)/bin
else
INSTALL_GOBIN=$(shell go env GOBIN)
endif

CONTROLLER_GEN_VER := v0.10.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)
export CONTROLLER_GEN # so hack scripts can use it

YAML_PATCH_VER ?= v0.0.11
YAML_PATCH_BIN := yaml-patch
YAML_PATCH := $(TOOLS_DIR)/$(YAML_PATCH_BIN)-$(YAML_PATCH_VER)
export YAML_PATCH # so hack scripts can use it

OPENSHIFT_GOIMPORTS_VER := c72f1dc2e3aacfa00aece3391d938c9bc734e791
OPENSHIFT_GOIMPORTS_BIN := openshift-goimports
OPENSHIFT_GOIMPORTS := $(TOOLS_DIR)/$(OPENSHIFT_GOIMPORTS_BIN)-$(OPENSHIFT_GOIMPORTS_VER)
export OPENSHIFT_GOIMPORTS # so hack scripts can use it

GOLANGCI_LINT_VER := v1.49.0
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_GOBIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

STATICCHECK_VER := 2022.1
STATICCHECK_BIN := staticcheck
STATICCHECK := $(TOOLS_GOBIN_DIR)/$(STATICCHECK_BIN)-$(STATICCHECK_VER)

GOTESTSUM_VER := v1.8.1
GOTESTSUM_BIN := gotestsum
GOTESTSUM := $(abspath $(TOOLS_DIR))/$(GOTESTSUM_BIN)-$(GOTESTSUM_VER)

LOGCHECK_VER := v0.2.0
LOGCHECK_BIN := logcheck
LOGCHECK := $(TOOLS_GOBIN_DIR)/$(LOGCHECK_BIN)-$(LOGCHECK_VER)
export LOGCHECK # so hack scripts can use it

ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)

LDFLAGS :=
all: build
.PHONY: all

ldflags:
	@echo $(LDFLAGS)

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

build: WHAT ?= ./cmd/...
build: require-jq require-go require-git verify-go-versions ## Build the project
	GOOS=$(OS) GOARCH=$(ARCH) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o bin/ $(WHAT)
.PHONY: build

.PHONY: build-all
build-all:
	GOOS=$(OS) GOARCH=$(ARCH) $(MAKE) build WHAT=./cmd/...

install: WHAT ?= ./cmd/...
install: ## install binaries to GOBIN
	GOOS=$(OS) GOARCH=$(ARCH) go install -ldflags="$(LDFLAGS)" $(WHAT)
.PHONY: install

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(STATICCHECK):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) honnef.co/go/tools/cmd/staticcheck $(STATICCHECK_BIN) $(STATICCHECK_VER)

$(LOGCHECK):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) sigs.k8s.io/logtools/logcheck $(LOGCHECK_BIN) $(LOGCHECK_VER)

lint: $(GOLANGCI_LINT) $(STATICCHECK) $(LOGCHECK) ## Run linters
	$(GOLANGCI_LINT) run --timeout=10m --skip-dirs pkg/client ./...
	$(STATICCHECK) -checks ST1019,ST1005 ./...
.PHONY: lint

vendor: ## Vendor the dependencies
	go mod tidy
	go mod vendor
.PHONY: vendor

tools: $(GOLANGCI_LINT) $(CONTROLLER_GEN) $(YAML_PATCH) $(GOTESTSUM) $(OPENSHIFT_GOIMPORTS)
.PHONY: tools

$(CONTROLLER_GEN):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

$(YAML_PATCH):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/pivotal-cf/yaml-patch/cmd/yaml-patch $(YAML_PATCH_BIN) $(YAML_PATCH_VER)

$(GOTESTSUM):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) gotest.tools/gotestsum $(GOTESTSUM_BIN) $(GOTESTSUM_VER)

codegen: $(CONTROLLER_GEN) $(YAML_PATCH) ## Run the codegenerators
	go mod download
	./hack/update-codegen.sh
	$(MAKE) imports
.PHONY: codegen

# Note, running this locally if you have any modified files, even those that are not generated,
# will result in an error. This target is mostly for CI jobs.
.PHONY: verify-codegen
verify-codegen:
	if [[ -n "${GITHUB_WORKSPACE}" ]]; then \
		mkdir -p $$(go env GOPATH)/src/github.com/kube-bind; \
		ln -s ${GITHUB_WORKSPACE} $$(go env GOPATH)/src/github.com/kube-bind/kube-bind; \
	fi

	$(MAKE) codegen

	if ! git diff --quiet HEAD; then \
		git diff; \
		echo "You need to run 'make codegen' to update generated files and commit them"; \
		exit 1; \
	fi

$(OPENSHIFT_GOIMPORTS):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/openshift-eng/openshift-goimports $(OPENSHIFT_GOIMPORTS_BIN) $(OPENSHIFT_GOIMPORTS_VER)

.PHONY: imports
imports: $(OPENSHIFT_GOIMPORTS)
	$(OPENSHIFT_GOIMPORTS) -m github.com/kube-bind/kube-bind

$(TOOLS_DIR)/verify_boilerplate.py:
	mkdir -p $(TOOLS_DIR)
	curl --fail --retry 3 -L -o $(TOOLS_DIR)/verify_boilerplate.py https://raw.githubusercontent.com/kubernetes/repo-infra/master/hack/verify_boilerplate.py
	chmod +x $(TOOLS_DIR)/verify_boilerplate.py

.PHONY: verify-boilerplate
verify-boilerplate: $(TOOLS_DIR)/verify_boilerplate.py
	$(TOOLS_DIR)/verify_boilerplate.py --boilerplate-dir=hack/boilerplate

ifdef ARTIFACT_DIR
GOTESTSUM_ARGS += --junitfile=$(ARTIFACT_DIR)/junit.xml
endif

GO_TEST = go test
ifdef USE_GOTESTSUM
GO_TEST = $(GOTESTSUM) $(GOTESTSUM_ARGS) --
endif

COUNT ?= 1
E2E_PARALLELISM ?= 1

.PHONY: test-e2e
ifdef USE_GOTESTSUM
test-e2e: $(GOTESTSUM)
endif
test-e2e: TEST_ARGS ?=
test-e2e: WHAT ?= ./test/e2e...
test-e2e: build-all ## run e2e tests
	NO_GORUN=1 GOOS=$(OS) GOARCH=$(ARCH) $(GO_TEST) -race -count $(COUNT) -p $(E2E_PARALLELISM) -parallel $(E2E_PARALLELISM) $(WHAT) $(TEST_ARGS)

.PHONY: test
ifdef USE_GOTESTSUM
test: $(GOTESTSUM)
endif
test: WHAT ?= ./...
# We will need to move into the sub package, of pkg/apis to run those tests.
test:  ## run unit tests
	$(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) $$(go list "$(WHAT)" | grep -v ./test/e2e/)
	cd pkg/apis && $(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) $(WHAT)

.PHONY: verify-imports
verify-imports:
	hack/verify-imports.sh

.PHONY: verify-go-versions
verify-go-versions:
	hack/verify-go-versions.sh

.PHONY: modules
modules: ## Run go mod tidy to ensure modules are up to date
	go mod tidy
	cd pkg/apis; go mod tidy

.PHONY: verify-modules
verify-modules: modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod pkg/apis/go.mod pkg/apis/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi

.PHONY: verify
verify: verify-modules verify-go-versions verify-imports verify-codegen ## verify formal properties of the code

.PHONY: help
help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

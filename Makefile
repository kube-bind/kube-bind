# Copyright 2021 The Kube Bind Authors.
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
ROOT_DIR=$(abspath .)
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

CONTROLLER_GEN_VER := v0.16.5
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)
export CONTROLLER_GEN # so hack scripts can use it

KUBE_CLIENT_GEN_VER := v0.31.0
KUBE_CLIENT_GEN_BIN := client-gen
KUBE_LISTER_GEN_VER := v0.31.0
KUBE_LISTER_GEN_BIN := lister-gen
KUBE_INFORMER_GEN_VER := v0.31.0
KUBE_INFORMER_GEN_BIN := informer-gen
KUBE_APPLYCONFIGURATION_GEN_VER := v0.31.0
KUBE_APPLYCONFIGURATION_GEN_BIN := applyconfiguration-gen

KUBE_CLIENT_GEN := $(GOBIN_DIR)/$(KUBE_CLIENT_GEN_BIN)-$(KUBE_CLIENT_GEN_VER)
export KUBE_CLIENT_GEN
KUBE_LISTER_GEN := $(GOBIN_DIR)/$(KUBE_LISTER_GEN_BIN)-$(KUBE_LISTER_GEN_VER)
export KUBE_LISTER_GEN
KUBE_INFORMER_GEN := $(GOBIN_DIR)/$(KUBE_INFORMER_GEN_BIN)-$(KUBE_INFORMER_GEN_VER)
export KUBE_INFORMER_GEN
KUBE_APPLYCONFIGURATION_GEN := $(GOBIN_DIR)/$(KUBE_APPLYCONFIGURATION_GEN_BIN)-$(KUBE_APPLYCONFIGURATION_GEN_VER)
export KUBE_APPLYCONFIGURATION_GEN

YAML_PATCH_VER ?= v0.0.11
YAML_PATCH_BIN := yaml-patch
YAML_PATCH := $(TOOLS_DIR)/$(YAML_PATCH_BIN)-$(YAML_PATCH_VER)
export YAML_PATCH # so hack scripts can use it

OPENSHIFT_GOIMPORTS_VER := c72f1dc2e3aacfa00aece3391d938c9bc734e791
OPENSHIFT_GOIMPORTS_BIN := openshift-goimports
OPENSHIFT_GOIMPORTS := $(TOOLS_DIR)/$(OPENSHIFT_GOIMPORTS_BIN)-$(OPENSHIFT_GOIMPORTS_VER)
export OPENSHIFT_GOIMPORTS # so hack scripts can use it

GOLANGCI_LINT_VER := v1.63.1
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_GOBIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

GOTESTSUM_VER := v1.8.1
GOTESTSUM_BIN := gotestsum
GOTESTSUM := $(abspath $(TOOLS_DIR))/$(GOTESTSUM_BIN)-$(GOTESTSUM_VER)

LOGCHECK_VER := v0.2.0
LOGCHECK_BIN := logcheck
LOGCHECK := $(TOOLS_GOBIN_DIR)/$(LOGCHECK_BIN)-$(LOGCHECK_VER)
export LOGCHECK # so hack scripts can use it

CODE_GENERATOR_VER := v2.3.1
CODE_GENERATOR_BIN := code-generator
CODE_GENERATOR := $(TOOLS_GOBIN_DIR)/$(CODE_GENERATOR_BIN)-$(CODE_GENERATOR_VER)
export CODE_GENERATOR # so hack scripts can use it

KCP_VER := v0.25.0
KCP_BIN := kcp
KCP := $(TOOLS_GOBIN_DIR)/$(KCP_BIN)-$(KCP_VER)

DEX_VER := v2.41.1
DEX_BIN := dex
DEX := $(TOOLS_GOBIN_DIR)/$(DEX_BIN)-$(DEX_VER)

ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)

KUBE_MAJOR_VERSION := 1
KUBE_MINOR_VERSION := $(shell go mod edit -json | jq '.Require[] | select(.Path == "k8s.io/client-go") | .Version' --raw-output | sed "s/v[0-9]*\.\([0-9]*\).*/\1/")
GIT_COMMIT := $(shell git rev-parse --short HEAD || echo 'local')
GIT_DIRTY := $(shell git diff --quiet && echo 'clean' || echo 'dirty')
GIT_VERSION := $(shell go mod edit -json | jq '.Require[] | select(.Path == "k8s.io/client-go") | .Version' --raw-output | sed 's/v0/v1/')+kube-bind-$(shell git describe --tags --match='v*' --abbrev=14 "$(GIT_COMMIT)^{commit}" 2>/dev/null || echo v0.0.0-$(GIT_COMMIT))
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := \
	-X k8s.io/client-go/pkg/version.gitCommit=${GIT_COMMIT} \
	-X k8s.io/client-go/pkg/version.gitTreeState=${GIT_DIRTY} \
	-X k8s.io/client-go/pkg/version.gitVersion=${GIT_VERSION} \
	-X k8s.io/client-go/pkg/version.gitMajor=${KUBE_MAJOR_VERSION} \
	-X k8s.io/client-go/pkg/version.gitMinor=${KUBE_MINOR_VERSION} \
	-X k8s.io/client-go/pkg/version.buildDate=${BUILD_DATE} \
	\
	-X k8s.io/component-base/version.gitCommit=${GIT_COMMIT} \
	-X k8s.io/component-base/version.gitTreeState=${GIT_DIRTY} \
	-X k8s.io/component-base/version.gitVersion=${GIT_VERSION} \
	-X k8s.io/component-base/version.gitMajor=${KUBE_MAJOR_VERSION} \
	-X k8s.io/component-base/version.gitMinor=${KUBE_MINOR_VERSION} \
	-X k8s.io/component-base/version.buildDate=${BUILD_DATE}
all: build
.PHONY: all

ldflags:
	@echo $(LDFLAGS)

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

build: WHAT ?= ./cmd/... ./cli/cmd/...
build: require-jq require-go require-git verify-go-versions ## Build the project
	set -x; for W in $(WHAT); do \
		pushd . && cd $${W%..}; \
    	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(ROOT_DIR)/bin ./...; \
		popd; \
    done
.PHONY: build

.PHONY: build-all
build-all:
	GOOS=$(OS) GOARCH=$(ARCH) $(MAKE) build WHAT=./cmd/...

install: WHAT ?= ./cmd/... ./cli/cmd/...
install: ## install binaries to GOBIN
	GOOS=$(OS) GOARCH=$(ARCH) go install -ldflags="$(LDFLAGS)" $(WHAT)
.PHONY: install

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(LOGCHECK):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) sigs.k8s.io/logtools/logcheck $(LOGCHECK_BIN) $(LOGCHECK_VER)

$(CODE_GENERATOR):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/kcp-dev/code-generator/v2 $(CODE_GENERATOR_BIN) $(CODE_GENERATOR_VER)

lint: $(GOLANGCI_LINT) $(LOGCHECK) ## Run linters
	$(GOLANGCI_LINT) run ./...
.PHONY: lint

vendor: ## Vendor the dependencies
	go mod tidy
	go mod vendor
.PHONY: vendor

tools: $(GOLANGCI_LINT) $(CONTROLLER_GEN) $(YAML_PATCH) $(GOTESTSUM) $(OPENSHIFT_GOIMPORTS) $(CODE_GENERATOR)
.PHONY: tools

$(CONTROLLER_GEN):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

$(YAML_PATCH):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/pivotal-cf/yaml-patch/cmd/yaml-patch $(YAML_PATCH_BIN) $(YAML_PATCH_VER)

$(GOTESTSUM):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) gotest.tools/gotestsum $(GOTESTSUM_BIN) $(GOTESTSUM_VER)

$(KUBE_CLIENT_GEN):
	GOBIN=$(GOBIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/$(KUBE_CLIENT_GEN_BIN) $(KUBE_CLIENT_GEN_BIN) $(KUBE_CLIENT_GEN_VER)
$(KUBE_LISTER_GEN):
	GOBIN=$(GOBIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/$(KUBE_LISTER_GEN_BIN) $(KUBE_LISTER_GEN_BIN) $(KUBE_LISTER_GEN_VER)
$(KUBE_INFORMER_GEN):
	GOBIN=$(GOBIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/$(KUBE_INFORMER_GEN_BIN) $(KUBE_INFORMER_GEN_BIN) $(KUBE_INFORMER_GEN_VER)
$(KUBE_APPLYCONFIGURATION_GEN):
	GOBIN=$(GOBIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/$(KUBE_APPLYCONFIGURATION_GEN_BIN) $(KUBE_APPLYCONFIGURATION_GEN_BIN) $(KUBE_APPLYCONFIGURATION_GEN_VER)


codegen: $(CONTROLLER_GEN) $(YAML_PATCH) $(CODE_GENERATOR) $(KUBE_CLIENT_GEN) $(KUBE_LISTER_GEN) $(KUBE_INFORMER_GEN) $(KUBE_APPLYCONFIGURATION_GEN) ## Run the codegenerators
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
	$(TOOLS_DIR)/verify_boilerplate.py --boilerplate-dir=hack/boilerplate --skip dex

ifdef ARTIFACT_DIR
GOTESTSUM_ARGS += --junitfile=$(ARTIFACT_DIR)/junit.xml
endif

GO_TEST = go test
ifdef USE_GOTESTSUM
GO_TEST = $(GOTESTSUM) $(GOTESTSUM_ARGS) --
endif

COUNT ?= 1
E2E_PARALLELISM ?= 1

dex:
		git clone https://github.com/dexidp/dex.git
dex/bin/dex: dex
		cd dex
		make

$(DEX):
	mkdir -p $(TOOLS_DIR)
	git clone --branch $(DEX_VER) --depth 1 https://github.com/dexidp/dex $(TOOLS_DIR)/dex-clone-$(DEX_VER)
	cd $(TOOLS_DIR)/dex-clone-$(DEX_VER) && make build
	cp -a $(TOOLS_DIR)/dex-clone-$(DEX_VER)/bin/dex $(DEX)

$(KCP):
	mkdir -p $(TOOLS_DIR)
	curl --fail --retry 3 -L "https://github.com/kcp-dev/kcp/releases/download/$(KCP_VER)/kcp_$(KCP_VER:v%=%)_$(OS)_$(ARCH).tar.gz" | \
	tar xz -C "$(TOOLS_DIR)" --strip-components="1" bin/kcp
	mv $(TOOLS_DIR)/kcp $(KCP)

.PHONY: test-e2e
ifdef USE_GOTESTSUM
test-e2e: $(GOTESTSUM)
endif
test-e2e: TEST_ARGS ?=
test-e2e: WORK_DIR ?= .
test-e2e: WHAT ?= ./test/e2e...
test-e2e: $(KCP) $(DEX) build-all
	mkdir .kcp
	$(DEX) serve hack/dex-config-dev.yaml 2>&1 & DEX_PID=$$!; \
	$(KCP) start &>.kcp/kcp.log & KCP_PID=$$!; \
	trap 'kill -TERM $$DEX_PID $$KCP_PID; rm -rf .kcp' TERM INT EXIT && \
	echo "Waiting for kcp to be ready (check .kcp/kcp.log)." && while ! KUBECONFIG=.kcp/admin.kubeconfig kubectl get --raw /readyz &>/dev/null; do sleep 1; echo -n "."; done && echo && \
	KUBECONFIG=$$PWD/.kcp/admin.kubeconfig GOOS=$(OS) GOARCH=$(ARCH) $(GO_TEST) -race -count $(COUNT) -p $(E2E_PARALLELISM) -parallel $(E2E_PARALLELISM) $(WHAT) $(TEST_ARGS)

.PHONY: test
ifdef USE_GOTESTSUM
test: $(GOTESTSUM)
endif
test: WHAT ?= ./...
# We will need to move into the sub package, of pkg/apis to run those tests.
test:  ## run unit tests
	$(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) $$(go list "$(WHAT)" | grep -v ./test/e2e/)
	cd sdk/apis && $(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) $(WHAT)

.PHONY: verify-imports
verify-imports:
	hack/verify-imports.sh

.PHONY: verify-go-versions
verify-go-versions:
	hack/verify-go-versions.sh

.PHONY: modules
modules: ## Run go mod tidy to ensure modules are up to date
	go mod tidy
	cd sdk/apis; go mod tidy

.PHONY: verify-modules
verify-modules: modules  # Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod pkg/apis/go.mod pkg/apis/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi

.PHONY: verify
verify: verify-modules verify-go-versions verify-imports verify-codegen verify-boilerplate ## verify formal properties of the code

.PHONY: help
help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: generate-cli-docs
generate-cli-docs: ## Generate cli docs
	git clean -fdX docs/content/reference/cli
	pushd . && cd docs/generators/cli-doc && go build . && popd
	./docs/generators/cli-doc/cli-doc -output docs/content/reference/cli

.PHONY: generate-api-docs
generate-api-docs: ## Generate api docs
	git clean -fdX docs/content/reference/api
	docs/generators/crd-ref/run-crd-ref-gen.sh

VENVDIR=$(abspath docs/venv)
REQUIREMENTS_TXT=docs/requirements.txt

.PHONY: serve-docs
serve-docs: venv ## Serve docs
	. $(VENV)/activate; \
	VENV=$(VENV) REMOTE=$(REMOTE) BRANCH=$(BRANCH) docs/scripts/serve-docs.sh

.PHONY: deploy-docs
deploy-docs: venv ## Deploy docs
	. $(VENV)/activate; \
	REMOTE=$(REMOTE) BRANCH=$(BRANCH) docs/scripts/deploy-docs.sh

include Makefile.venv

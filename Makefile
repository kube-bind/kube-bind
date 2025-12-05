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

ROOT_DIR = $(abspath .)
TOOLS_DIR = hack/tools
BUILD_DIR = $(ROOT_DIR)/bin

export UGET_DIRECTORY = $(TOOLS_DIR)
export UGET_CHECKSUMS = hack/tools.checksums
export UGET_VERSIONED_BINARIES = true

# Image build configuration
# REV is the short git sha of latest commit.
export REV ?= $(shell git rev-parse --short HEAD)
export IMAGE_REPO ?= kube-bind

BOILERPLATE_VERSION := 201dcad9616c117927232ee0bc499ff38a27023e
CODE_GENERATOR_VERSION := v2.4.0
CONTROLLER_GEN_VERSION := v0.17.3
DEX_VERSION := v2.43.1
GOLANGCI_LINT_VERSION := 2.1.6
GORELEASER_VERSION := 2.13.0
GOTESTSUM_VERSION := 1.8.1
HELM_VERSION := 3.18.6
KCP_VERSION := 0.28.3
KUBE_APPLYCONFIGURATION_GEN_VERSION := v0.32.0
KUBE_CLIENT_GEN_VERSION := v0.32.0
KUBE_INFORMER_GEN_VERSION := v0.32.0
KUBE_LISTER_GEN_VERSION := v0.32.0
LOGCHECK_VERSION := v0.2.0
YAML_PATCH_VERSION ?= v0.0.11

KUBE_MAJOR_VERSION := 1
KUBE_MINOR_VERSION := $(shell go mod edit -json | jq '.Require[] | select(.Path == "k8s.io/client-go") | .Version' --raw-output | sed "s/v[0-9]*\.\([0-9]*\).*/\1/")
GIT_COMMIT := $(shell git rev-parse --short HEAD || echo 'local')
# --quiet would still produces output when files are deleted
GIT_DIRTY := $(shell git diff --quiet >/dev/null && echo 'clean' || echo 'dirty')
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

CONTRIBS ?= $(patsubst contrib/%,%,$(wildcard contrib/*))

.PHONY: all
all: build

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	@echo "Cleaned $(BUILD_DIR)."

.PHONY: clean-tools
clean-tools:
	rm -rf $(UGET_DIRECTORY)
	@echo "Cleaned $(UGET_DIRECTORY)."

.PHONY: check
check: verify lint test test-e2e test-e2e-contribs

GOMODS := $(shell find . -name 'go.mod' -exec dirname {} \; | grep -v hack/tools | grep -v ./dex)

.PHONY: ldflags
ldflags:
	@echo $(LDFLAGS)

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

.PHONY: build
build: WHAT ?= ./cmd/... ./cli/cmd/... ./contrib/kcp/cmd/kcp-init/...
build: export CGO_ENABLED=0
build: verify-go-versions ## Build the project
	@mkdir -p $(BUILD_DIR)
	@for W in $(WHAT); do \
		(set -x; cd $${W%...}; go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/ ./...); \
	done

.PHONY: install
install: WHAT ?= ./cmd/... ./cli/cmd/...
install: ## install binaries to GOBIN
	go install -ldflags="$(LDFLAGS)" $(WHAT)

GOLANGCI_LINT = $(ROOT_DIR)/$(UGET_DIRECTORY)/golangci-lint-$(GOLANGCI_LINT_VERSION)

.PHONY: install-golangci-lint
install-golangci-lint:
	@hack/uget.sh https://github.com/golangci/golangci-lint/releases/download/v{VERSION}/golangci-lint-{VERSION}-{GOOS}-{GOARCH}.tar.gz golangci-lint $(GOLANGCI_LINT_VERSION)

.PHONY: install-logcheck
install-logcheck:
	@GO_MODULE=true hack/uget.sh sigs.k8s.io/logtools/logcheck logcheck $(LOGCHECK_VERSION)

# .PHONY: install-kcp-codegen
# install-kcp-codegen:
# 	@GO_MODULE=true hack/uget.sh github.com/kcp-dev/code-generator/v2 kcp-code-generator $(CODE_GENERATOR_VERSION) code-generator

.PHONY: install-controller-gen
install-controller-gen:
	@GO_MODULE=true hack/uget.sh sigs.k8s.io/controller-tools/cmd/controller-gen controller-gen $(CONTROLLER_GEN_VERSION)

.PHONY: install-yaml-patch
install-yaml-patch:
	@GO_MODULE=true hack/uget.sh github.com/pivotal-cf/yaml-patch/cmd/yaml-patch yaml-patch $(YAML_PATCH_VERSION)

.PHONY: install-gotestsum
install-gotestsum:
	@hack/uget.sh https://github.com/gotestyourself/gotestsum/releases/download/v{VERSION}/gotestsum_{VERSION}_{GOOS}_{GOARCH}.tar.gz gotestsum $(GOTESTSUM_VERSION) gotestsum

# .PHONY: install-client-gen
# install-client-gen:
# 	@GO_MODULE=true hack/uget.sh k8s.io/code-generator/cmd/client-gen client-gen $(CLIENT_GEN_VERSION)

# .PHONY: install-lister-gen
# install-lister-gen:
# 	@GO_MODULE=true hack/uget.sh k8s.io/code-generator/cmd/lister-gen lister-gen $(CLIENT_GEN_VERSION)

# .PHONY: install-informer-gen
# install-informer-gen:
# 	@GO_MODULE=true hack/uget.sh k8s.io/code-generator/cmd/informer-gen informer-gen $(CLIENT_GEN_VERSION)

# .PHONY: install-applyconfiguration-gen
# install-applyconfiguration-gen:
# 	@GO_MODULE=true hack/uget.sh k8s.io/code-generator/cmd/applyconfiguration-gen applyconfiguration-gen $(APPLYCONFIGURATION_GEN_VERSION)

.PHONY: install-boilerplate
install-boilerplate:
	@UGET_VERSIONED_BINARIES=false UNCOMPRESSED=true hack/uget.sh https://raw.githubusercontent.com/kubernetes/repo-infra/master/hack/verify_boilerplate.py verify_boilerplate.py $(BOILERPLATE_VERSION) verify_boilerplate.py

.PHONY: install-kcp
install-kcp:
	@hack/uget.sh https://github.com/kcp-dev/kcp/releases/download/v{VERSION}/kcp_{VERSION}_{GOOS}_{GOARCH}.tar.gz kcp $(KCP_VERSION)

GORELEASER = $(UGET_DIRECTORY)/goreleaser-$(GORELEASER_VERSION)

.PHONY: install-goreleaser
install-goreleaser: export OS ?= $(shell uname -s)
install-goreleaser: export ARCH ?= $(shell uname -m)
install-goreleaser:
	@hack/uget.sh https://github.com/goreleaser/goreleaser/releases/download/v{VERSION}/goreleaser_{ENV:OS}_{ENV:ARCH}.tar.gz goreleaser $(GORELEASER_VERSION) goreleaser

.PHONY: install-helm
install-helm:
	@hack/uget.sh https://get.helm.sh/helm-v{VERSION}-{GOOS}-{GOARCH}.tar.gz helm $(HELM_VERSION)

# e2e tests use this env name to locate the dex binary; make sure it's an absolute path
export DEX_BINARY = $(ROOT_DIR)/$(UGET_DIRECTORY)/dex-$(DEX_VERSION)

.PHONY: install-dex
install-dex: $(DEX_BINARY)

$(DEX_BINARY):
	mkdir -p $(TOOLS_DIR)
	git clone --branch $(DEX_VERSION) --depth 1 https://github.com/dexidp/dex $(TOOLS_DIR)/dex-clone-$(DEX_VERSION) || true
	cd $(TOOLS_DIR)/dex-clone-$(DEX_VERSION) && GOWORK=off make build && cp bin/dex $(DEX_BINARY)

# This target can be used to conveniently update the checksums for all checksummed tools.
# Combine with GOARCH to update for other archs, like "GOARCH=arm64 make update-tools".

.PHONY: update-tools
update-tools: UGET_UPDATE=true
update-tools: clean-tools install-golangci-lint install-gotestsum install-boilerplate install-kcp install-helm install-goreleaser

.PHONY: lint
lint: install-golangci-lint install-logcheck ## Run golangci-lint
	@if [ -n "$(WHAT)" ]; then \
		$(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) -c $(ROOT_DIR)/.golangci.yaml --timeout 20m $(WHAT); \
	else \
		for MOD in $(GOMODS); do \
			(cd $$MOD; echo "Linting $$MOD"; $(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) -c $(ROOT_DIR)/.golangci.yaml --timeout 20m); \
		done; \
	fi

.PHONY: fix-lint
fix-lint: install-golangci-lint ## Run golangci-lint with --fix
	GOLANGCI_LINT_FLAGS="--fix" $(MAKE) lint

.PHONY: codegen
codegen: ## Generate code
	go mod download
	./hack/update-codegen.sh
	./hack/update-codegen-clients.sh
	$(MAKE) imports

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

.PHONY: imports
imports: install-golangci-lint verify-go-versions ## Fix imports and format code
	@if [ -n "$(WHAT)" ]; then \
		$(GOLANGCI_LINT) fmt --enable gci $(WHAT); \
	else \
		for MOD in $(GOMODS); do \
			( cd $$MOD; $(GOLANGCI_LINT) fmt --enable gci -c $(ROOT_DIR)/.golangci.yaml; ) \
	  	done; \
	fi

.PHONY: verify-boilerplate
verify-boilerplate: install-boilerplate
	$(TOOLS_DIR)/verify_boilerplate.py --boilerplate-dir=hack/boilerplate \
	  --skip dex \
	  --skip docs/venv \
	  --skip docs/__pycache__ \
	  --skip hack/uget.sh

ifdef ARTIFACT_DIR
GOTESTSUM_ARGS += --junitfile=$(ARTIFACT_DIR)/junit.xml
endif

GO_TEST = go test
ifdef USE_GOTESTSUM
GO_TEST = $(GOTESTSUM) $(GOTESTSUM_ARGS) --
endif

COUNT ?= 1
# Only set parallelism if user specified E2E_PARALLELISM
ifdef E2E_PARALLELISM
E2E_PARALLELISM_FLAG := -p $(E2E_PARALLELISM) -parallel $(E2E_PARALLELISM)
else
E2E_PARALLELISM_FLAG :=
endif

.PHONY: run-dex
run-dex: $(DEX_BINARY)
	$(DEX_BINARY) serve hack/dex-config-dev.yaml

KCP = $(TOOLS_DIR)/kcp-$(KCP_VERSION)

.PHONY: run-kcp
run-kcp: install-kcp
	$(KCP) start --bind-address=127.0.0.1

.PHONY: run-kcp-infra
run-kcp-infra: install-kcp $(DEX_BINARY) ## Run KCP infrastructure for e2e tests (blocking)
	mkdir -p .kcp
	$(MAKE) run-dex 2>&1 & DEX_PID=$$!; \
	$(MAKE) run-kcp &>.kcp/kcp.log & KCP_PID=$$!; \
	trap 'kill -TERM $$DEX_PID $$KCP_PID; rm -rf .kcp' TERM INT EXIT && \
	echo "Waiting for kcp to be ready (check .kcp/kcp.log)." && while ! KUBECONFIG=.kcp/admin.kubeconfig kubectl get --raw /readyz &>/dev/null; do sleep 1; echo -n "."; done && echo && \
	echo "KCP is ready. Press Ctrl+C to stop." && \
	wait $$KCP_PID

.PHONY: test-e2e-only
ifdef USE_GOTESTSUM
test-e2e-only: $(GOTESTSUM)
endif
test-e2e-only: TEST_ARGS ?=
test-e2e-only: WORK_DIR ?= .
test-e2e-only: WHAT ?= ./test/e2e...
test-e2e-only: build ## Run e2e tests against existing KCP infrastructure
	KUBECONFIG=$$PWD/.kcp/admin.kubeconfig GOOS=$(OS) GOARCH=$(ARCH) $(GO_TEST) -race -v -count $(COUNT) $(E2E_PARALLELISM_FLAG) $(WHAT) $(TEST_ARGS)

.PHONY: test-e2e
ifdef USE_GOTESTSUM
test-e2e: $(GOTESTSUM)
endif
test-e2e: TEST_ARGS ?=
test-e2e: WORK_DIR ?= .
test-e2e: TEST_WHAT ?= ./test/e2e...
test-e2e: $(DEX_BINARY) build ## Run e2e tests
	mkdir .kcp
	$(MAKE) run-kcp &>.kcp/kcp.log & KCP_PID=$$!; \
	trap 'kill -TERM $$KCP_PID; rm -rf .kcp' TERM INT EXIT && \
	echo "Waiting for kcp to be ready (check .kcp/kcp.log)." && while ! KUBECONFIG=.kcp/admin.kubeconfig kubectl get --raw /readyz &>/dev/null; do sleep 1; echo -n "."; done && echo && \
	KUBECONFIG=$$PWD/.kcp/admin.kubeconfig GOOS=$(OS) GOARCH=$(ARCH) $(GO_TEST) -race -count $(COUNT) $(E2E_PARALLELISM_FLAG) $(TEST_WHAT) $(TEST_ARGS)

CONTRIBS_E2E := $(patsubst %,test-e2e-contrib-%,$(CONTRIBS))

.PHONY: test-e2e-contribs $(CONTRIBS_E2E)
test-e2e-contribs: $(CONTRIBS_E2E) ## Run e2e tests for external integrations

.PHONY: test-e2e-contrib-kcp
test-e2e-contrib-kcp: $(DEX_BINARY)
$(CONTRIBS_E2E):
	mkdir .kcp
	$(MAKE) run-kcp &>.kcp/kcp.log & KCP_PID=$$!; \
	trap 'kill -TERM $$KCP_PID; rm -rf .kcp' TERM INT EXIT && \
	echo "Waiting for kcp to be ready (check .kcp/kcp.log)." && while ! KUBECONFIG=.kcp/admin.kubeconfig kubectl get --raw /readyz &>/dev/null; do sleep 1; echo -n "."; done && echo && \
	cd contrib/$(patsubst test-e2e-contrib-%,%,$@) && KUBECONFIG=$$PWD/../../.kcp/admin.kubeconfig $(GO_TEST) -race -count $(COUNT) $(E2E_PARALLELISM_FLAG) ./test/e2e/...

.PHONY: test
ifdef USE_GOTESTSUM
test: $(GOTESTSUM)
endif
test: TEST_WHAT ?= ./...
# We will need to move into the sub package, of pkg/apis to run those tests.
test:  ## Run unit tests
	@if [ -n "$(TEST_WHAT)" ]; then \
		$(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) $$(go list "$(TEST_WHAT)" | grep -v ./test/e2e/); \
	else \
		for MOD in $(GOMODS); do \
			( cd $$MOD; $(GO_TEST) -race -count $(COUNT) -coverprofile=coverage.txt -covermode=atomic $(TEST_ARGS) ) \
	  	done; \
	fi

.PHONY: verify-imports
verify-imports: imports
	if ! git diff --quiet HEAD; then \
		git diff; \
		echo "You need to run 'make imports' to update the immport statement ordering and commit them"; \
		exit 1; \
	fi

.PHONY: verify-go-versions
verify-go-versions:
	hack/verify-go-versions.sh

.PHONY: modules
modules: ## Run go mod tidy to ensure modules are up to date
	for MOD in $(GOMODS); do \
		(cd $$MOD; echo "Tidying $$MOD"; go mod tidy); \
	done

.PHONY: verify-modules
verify-modules: modules  # Verify go modules are up to date
	@for MOD in $(GOMODS); do \
		(cd $$MOD; echo "Verifying $$MOD"; if ! git diff --quiet HEAD -- go.mod go.sum; then git diff -- go.mod go.sum; echo "[$$MOD] go modules are out of date, please run 'make modules'"; exit 1; fi; ) \
	done

.PHONY: verify
verify: verify-go-versions verify-modules verify-imports verify-codegen verify-boilerplate ## verify formal properties of the code

.PHONY: help
help: ## Show this help
	@awk 'BEGIN { fs="## " } { FS=fs } /:.*##/ { doc=$$2; FS=":"; $$0=$$0; printf "\033[36m%-30s\033[0m %s\n", $$1, doc; }' $(MAKEFILE_LIST) | sort | grep -v fs=

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

.PHONY: local-docs
local-docs: venv ## Run mkdocs serve
	. $(VENV)/activate; \
	VENV=$(VENV) cd docs && mkdocs serve

.PHONY: serve-docs
serve-docs: venv ## Serve docs
	. $(VENV)/activate; \
	VENV=$(VENV) REMOTE=$(REMOTE) BRANCH=$(BRANCH) docs/scripts/serve-docs.sh

.PHONY: deploy-docs
deploy-docs: venv ## Deploy docs
	. $(VENV)/activate; \
	REMOTE=$(REMOTE) BRANCH=$(BRANCH) docs/scripts/deploy-docs.sh

.PHONY: build-web
build-web:
	cd web && npm run build

# Example: make IMAGE_REPO=ghcr.io/<username> image-local
# Set PLATFORMS to override default architectures (e.g., make PLATFORMS=linux/amd64,linux/arm64 image-local)
# For local builds, default to current architecture on Linux platform to support --load
.PHONY: image-local
image-local: export PLATFORMS ?= linux/$(shell go env GOARCH)
image-local:
	@LDFLAGS="$(LDFLAGS)" hack/build-image.sh

# Kind cluster configuration
KIND_CLUSTER ?= kube-bind
DOCKER_REPO ?= $(IMAGE_REPO)

.PHONY: kind-load
kind-load:
	@echo "Loading images into kind cluster '$(KIND_CLUSTER)'"
	kind load docker-image $(DOCKER_REPO)/konnector:$(REV) --name $(KIND_CLUSTER)
	kind load docker-image $(DOCKER_REPO)/backend:$(REV) --name $(KIND_CLUSTER)
	@echo "Successfully loaded images into kind cluster '$(KIND_CLUSTER)'"

.PHONY: helm-build-local
helm-build-local: ## Build and package Helm charts locally for testing
	@hack/helm-build.sh

.PHONY: helm-clean
helm-clean: ## Clean up built helm charts
	rm -f ./bin/*.tgz

.PHONY: goreleaser-test
goreleaser-test: install-goreleaser ## Test GoReleaser flow locally
	LDFLAGS="$(LDFLAGS)" $(GORELEASER) release --snapshot --clean

.PHONY: helm-push-local
helm-push-local: ## Push Helm charts to IMAGE_REPO registry
	@hack/helm-push.sh

.PHONY: helm-test
helm-test: helm-build-local ## Test Helm chart installation (dry-run)
	@hack/helm-test.sh

include Makefile.venv

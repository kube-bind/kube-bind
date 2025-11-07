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

ROOT_DIR=$(abspath .)
TOOLS_DIR=hack/tools
TOOLS_GOBIN_DIR := $(abspath $(TOOLS_DIR))
GOBIN_DIR=$(abspath ./bin )
PATH := $(GOBIN_DIR):$(TOOLS_GOBIN_DIR):$(PATH)
TMPDIR := $(shell mktemp -d)

# Image build configuration
# REV is the short git sha of latest commit.
REV ?= $(shell git rev-parse --short HEAD)
IMAGE_REPO ?= kube-bind

# Detect the path used for the install target
ifeq (,$(shell go env GOBIN))
INSTALL_GOBIN=$(shell go env GOPATH)/bin
else
INSTALL_GOBIN=$(shell go env GOBIN)
endif

CONTROLLER_GEN_VER := v0.17.3
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)
export CONTROLLER_GEN # so hack scripts can use it

KUBE_CLIENT_GEN_VER := v0.32.0
KUBE_CLIENT_GEN_BIN := client-gen
KUBE_LISTER_GEN_VER := v0.32.0
KUBE_LISTER_GEN_BIN := lister-gen
KUBE_INFORMER_GEN_VER := v0.32.0
KUBE_INFORMER_GEN_BIN := informer-gen
KUBE_APPLYCONFIGURATION_GEN_VER := v0.32.0
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

GOLANGCI_LINT_VER := v2.1.6
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_GOBIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

GOTESTSUM_VER := v1.8.1
GOTESTSUM_BIN := gotestsum
GOTESTSUM := $(abspath $(TOOLS_DIR))/$(GOTESTSUM_BIN)-$(GOTESTSUM_VER)

LOGCHECK_VER := v0.2.0
LOGCHECK_BIN := logcheck
LOGCHECK := $(TOOLS_GOBIN_DIR)/$(LOGCHECK_BIN)-$(LOGCHECK_VER)
export LOGCHECK # so hack scripts can use it

CODE_GENERATOR_VER := v2.4.0
CODE_GENERATOR_BIN := code-generator
CODE_GENERATOR := $(TOOLS_GOBIN_DIR)/$(CODE_GENERATOR_BIN)-$(CODE_GENERATOR_VER)
export CODE_GENERATOR # so hack scripts can use it

KCP_VER := v0.28.3
KCP_BIN := kcp
KCP := $(TOOLS_GOBIN_DIR)/$(KCP_BIN)-$(KCP_VER)
KCP_CMD ?= $(KCP)

DEX_VER := v2.43.1
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

CONTRIBS ?= $(patsubst contrib/%,%,$(wildcard contrib/*))

all: build
.PHONY: all

check: verify lint test test-e2e test-e2e-contribs
.PHONY: check

GOMODS := $(shell find . -name 'go.mod' -exec dirname {} \; | grep -v hack/tools | grep -v ./dex)

ldflags:
	@echo $(LDFLAGS)

.PHONY: require-%
require-%:
	@if ! command -v $* 1> /dev/null 2>&1; then echo "$* not found in \$$PATH"; exit 1; fi

build: WHAT ?= ./cmd/... ./cli/cmd/... ./contrib/kcp/cmd/kcp-init/...
build: require-jq require-go require-git verify-go-versions ## Build the project
	mkdir -p $(GOBIN_DIR)
	set -x; for W in $(WHAT); do \
		pushd . && cd $${W%..}; \
		GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o  $(GOBIN_DIR) ./...; \
		popd; \
	done
.PHONY: build

install: WHAT ?= ./cmd/... ./cli/cmd/...
install: ## install binaries to GOBIN
	GOOS=$(OS) GOARCH=$(ARCH) go install -ldflags="$(LDFLAGS)" $(WHAT)
.PHONY: install


$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/v2/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(LOGCHECK):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) sigs.k8s.io/logtools/logcheck $(LOGCHECK_BIN) $(LOGCHECK_VER)

$(CODE_GENERATOR):
	GOBIN=$(TOOLS_GOBIN_DIR) $(GO_INSTALL) github.com/kcp-dev/code-generator/v2 $(CODE_GENERATOR_BIN) $(CODE_GENERATOR_VER)

lint: $(GOLANGCI_LINT) $(LOGCHECK) ## Run golangci-lint
	@if [ -n "$(WHAT)" ]; then \
		$(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) -c $(ROOT_DIR)/.golangci.yaml --timeout 20m $(WHAT); \
	else \
		for MOD in $(GOMODS); do \
			(cd $$MOD; echo "Linting $$MOD"; $(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) -c $(ROOT_DIR)/.golangci.yaml --timeout 20m); \
		done; \
	fi
.PHONY: lint

fix-lint: $(GOLANGCI_LINT) ## Run golangci-lint with --fix
	GOLANGCI_LINT_FLAGS="--fix" $(MAKE) lint
.PHONY: fix-lint

vendor: ## Vendor the dependencies
	go mod tidy
	go mod vendor
.PHONY: vendor

tools: $(GOLANGCI_LINT) $(CONTROLLER_GEN) $(YAML_PATCH) $(GOTESTSUM) $(CODE_GENERATOR) ## Install tools
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


codegen: $(CONTROLLER_GEN) $(YAML_PATCH) $(CODE_GENERATOR) $(KUBE_CLIENT_GEN) $(KUBE_LISTER_GEN) $(KUBE_INFORMER_GEN) $(KUBE_APPLYCONFIGURATION_GEN) ## Generate code
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

.PHONY: imports
imports: $(GOLANGCI_LINT) verify-go-versions ## Fix imports and format code
	@if [ -n "$(WHAT)" ]; then \
		$(GOLANGCI_LINT) fmt --enable gci -c $(ROOT_DIR)/.golangci.yaml $(WHAT); \
	else \
		for MOD in $(GOMODS); do \
			( cd $$MOD; $(GOLANGCI_LINT) fmt --enable gci -c $(ROOT_DIR)/.golangci.yaml; ) \
	  	done; \
	fi

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
# Only set parallelism if user specified E2E_PARALLELISM
ifdef E2E_PARALLELISM
E2E_PARALLELISM_FLAG := -p $(E2E_PARALLELISM) -parallel $(E2E_PARALLELISM)
else
E2E_PARALLELISM_FLAG :=
endif

$(DEX):
	mkdir -p $(TOOLS_DIR)
	git clone --branch $(DEX_VER) --depth 1 https://github.com/dexidp/dex $(TOOLS_DIR)/dex-clone-$(DEX_VER) || true
	cd $(TOOLS_DIR)/dex-clone-$(DEX_VER) && GOWORK=off make build
	cp -a $(TOOLS_DIR)/dex-clone-$(DEX_VER)/bin/dex $(DEX)
	ln -sf $(DEX) $(TOOLS_GOBIN_DIR)/dex

run-dex: $(DEX)
	$(DEX) serve hack/dex-config-dev.yaml

$(KCP):
	mkdir -p $(TOOLS_DIR)
	curl --fail --retry 3 -L "https://github.com/kcp-dev/kcp/releases/download/$(KCP_VER)/kcp_$(KCP_VER:v%=%)_$(OS)_$(ARCH).tar.gz" | \
	tar xz -C "$(TOOLS_DIR)" --strip-components="1" bin/kcp
	mv $(TOOLS_DIR)/kcp $(KCP)
	ln -sf $(KCP) $(TOOLS_GOBIN_DIR)/kcp

run-kcp: $(KCP)
	$(KCP_CMD) start --bind-address=127.0.0.1

.PHONY: run-kcp-infra
run-kcp-infra: $(KCP) $(DEX) ## Run KCP infrastructure for e2e tests (blocking)
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
test-e2e: $(KCP) $(DEX) build ## Run e2e tests
	mkdir .kcp
	$(MAKE) run-kcp &>.kcp/kcp.log & KCP_PID=$$!; \
	trap 'kill -TERM $$KCP_PID; rm -rf .kcp' TERM INT EXIT && \
	echo "Waiting for kcp to be ready (check .kcp/kcp.log)." && while ! KUBECONFIG=.kcp/admin.kubeconfig kubectl get --raw /readyz &>/dev/null; do sleep 1; echo -n "."; done && echo && \
	KUBECONFIG=$$PWD/.kcp/admin.kubeconfig GOOS=$(OS) GOARCH=$(ARCH) $(GO_TEST) -race -count $(COUNT) $(E2E_PARALLELISM_FLAG) $(TEST_WHAT) $(TEST_ARGS)

CONTRIBS_E2E := $(patsubst %,test-e2e-contrib-%,$(CONTRIBS))

.PHONY: test-e2e-contribs $(CONTRIBS_E2E)
test-e2e-contribs: $(CONTRIBS_E2E) ## Run e2e tests for external integrations

test-e2e-contrib-kcp: $(DEX) $(KCP)
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
verify: verify-modules verify-go-versions verify-imports verify-codegen verify-boilerplate ## verify formal properties of the code

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
PLATFORMS ?= linux/$(ARCH)
.PHONY: image-local
image-local:
	@echo "Building images locally with tag $(REV) for platforms: $(PLATFORMS)"
	@command -v docker >/dev/null 2>&1 || { echo "docker not found. Please install Docker"; exit 1; }
	@docker buildx version >/dev/null 2>&1 || { echo "docker buildx not found. Please enable buildx in Docker"; exit 1; }

	@# Create buildx builder if it doesn't exist
	@docker buildx create --name kube-bind-builder --use 2>/dev/null || docker buildx use kube-bind-builder 2>/dev/null || true
	@docker buildx inspect --bootstrap >/dev/null 2>&1

	@# Check if building for multiple platforms
	@if [[ "$(PLATFORMS)" == *","* ]]; then \
		echo "Multi-platform build detected. Images will be pushed to registry instead of loaded locally."; \
		LOAD_FLAG="--push"; \
	else \
		echo "Single platform build. Images will be loaded to local Docker daemon."; \
		LOAD_FLAG="--load"; \
	fi && \
	\
	echo "Building konnector image..." && \
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		-t $(IMAGE_REPO)/konnector:$(REV) \
		-f Dockerfile.konnector \
		$$LOAD_FLAG . && \
	\
	echo "Building backend image..." && \
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		-t $(IMAGE_REPO)/backend:$(REV) \
		-f Dockerfile \
		$$LOAD_FLAG . && \
	\
	echo "Successfully built images:" && \
	echo "  $(IMAGE_REPO)/konnector:$(REV) ($(PLATFORMS))" && \
	echo "  $(IMAGE_REPO)/backend:$(REV) ($(PLATFORMS))"

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
	@echo "Building Helm charts locally..."
	@command -v helm >/dev/null 2>&1 || { echo "helm not found. Install from: https://helm.sh/docs/intro/install/"; exit 1; }

	@# Set chart version to semver format for local builds (0.0.0-<git-sha>)
	CHART_VERSION="0.0.0-$(REV)"; \
	for chart_dir in deploy/charts/*/; do \
		if [ -f "$${chart_dir}Chart.yaml" ]; then \
			chart_name=$$(basename "$$chart_dir"); \
			echo "Processing chart: $$chart_name"; \
			\
			cp "$${chart_dir}Chart.yaml" "$${chart_dir}Chart.yaml.bak"; \
			sed -i.tmp "s/^version:.*/version: $$CHART_VERSION/" "$${chart_dir}Chart.yaml"; \
			sed -i.tmp "s/^appVersion:.*/appVersion: $$CHART_VERSION/" "$${chart_dir}Chart.yaml"; \
			rm -f "$${chart_dir}Chart.yaml.tmp"; \
			\
			helm package "$$chart_dir" --version "$$CHART_VERSION" --destination ./bin/; \
			echo "Packaged: ./bin/$$chart_name-$$CHART_VERSION.tgz"; \
			\
			mv "$${chart_dir}Chart.yaml.bak" "$${chart_dir}Chart.yaml"; \
		fi; \
	done
	@echo "Helm charts built successfully in ./bin/"

.PHONY: helm-clean
helm-clean: ## Clean up built helm charts
	rm -f ./bin/*.tgz

.PHONY: helm-push-local
helm-push-local: ## Push Helm charts to IMAGE_REPO registry
	@echo "Pushing Helm charts to registry: $(IMAGE_REPO)"
	@command -v helm >/dev/null 2>&1 || { echo "helm not found. Install from: https://helm.sh/docs/intro/install/"; exit 1; }

	CHART_VERSION="0.0.0-$(REV)"; \
	export HELM_EXPERIMENTAL_OCI=1; \
	for chart_file in ./bin/*-$$CHART_VERSION.tgz; do \
		if [ -f "$$chart_file" ]; then \
			chart_filename=$$(basename "$$chart_file"); \
			chart_name=$${chart_filename%-$$CHART_VERSION.tgz}; \
			if [[ "$$chart_name" =~ [[:space:]] ]]; then \
				echo "Skipping chart with invalid name: '$$chart_name' (contains spaces)"; \
				continue; \
			fi; \
			echo "Pushing $$chart_name to $(IMAGE_REPO)"; \
			helm push "$$chart_file" "oci://$(IMAGE_REPO)/charts"; \
			echo "Chart available at: oci://$(IMAGE_REPO)/charts/$$chart_name:$$CHART_VERSION"; \
		fi; \
	done

.PHONY: helm-test
helm-test: helm-build-local ## Test Helm chart installation (dry-run)
	@echo "Testing Helm chart installation..."
	CHART_VERSION="0.0.0-$(REV)"; \
	for chart_dir in deploy/charts/*/; do \
		if [ -f "$${chart_dir}Chart.yaml" ]; then \
			chart_name=$$(basename "$$chart_dir"); \
			echo "Testing chart: $$chart_name"; \
			helm install test-$$chart_name "./bin/$$chart_name-$$CHART_VERSION.tgz" --dry-run --debug; \
			echo "✓ Chart $$chart_name passes dry-run test"; \
		fi; \
	done

include Makefile.venv

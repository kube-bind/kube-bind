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

TOOLS_DIR=hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR))/bin
export TOOLS_BIN_DIR # so hack scripts can use it

GO_INSTALL = ./hack/go-install.sh

KCP_VERSION ?= 0.23.0
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
export CONTROLLER_GEN # so hack scripts can use it

CODE_GENERATOR_VER := v2.1.0
CODE_GENERATOR_BIN := code-generator
CODE_GENERATOR := $(TOOLS_BIN_DIR)/$(CODE_GENERATOR_BIN)-$(CODE_GENERATOR_VER)
export CODE_GENERATOR # so hack scripts can use itßß

KCP_APIGEN_VER := v0.26.0
KCP_APIGEN_BIN := apigen
KCP_APIGEN_GEN := $(TOOLS_BIN_DIR)/$(KCP_APIGEN_BIN)-$(KCP_APIGEN_VER)
export KCP_APIGEN_GEN # so hack scripts can use it

OPENSHIFT_GOIMPORTS_VER := c72f1dc2e3aacfa00aece3391d938c9bc734e791
OPENSHIFT_GOIMPORTS_BIN := openshift-goimports
OPENSHIFT_GOIMPORTS := $(TOOLS_BIN_DIR)/$(OPENSHIFT_GOIMPORTS_BIN)-$(OPENSHIFT_GOIMPORTS_VER)
export OPENSHIFT_GOIMPORTS # so hack scripts can use it

export KCP_REPO_DIR=${GOPATH}/src/github.com/kcp-dev/kcp/
export KCP_KUBECONFIG=${KCP_REPO_DIR}.kcp/admin.kubeconfig

$(KCP_APIGEN_GEN):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/kcp-dev/kcp/sdk/cmd/apigen $(KCP_APIGEN_BIN) $(KCP_APIGEN_VER)

$(CONTROLLER_GEN):  # Build controller-gen from tools folder.
	cd $(TOOLS_BIN_DIR) && go build -tags=tools -o bin/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(CODE_GENERATOR):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/kcp-dev/code-generator/v2 $(CODE_GENERATOR_BIN) $(CODE_GENERATOR_VER)

$(OPENSHIFT_GOIMPORTS):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/openshift-eng/openshift-goimports $(OPENSHIFT_GOIMPORTS_BIN) $(OPENSHIFT_GOIMPORTS_VER)


tools: $(CONTROLLER_GEN) $(KCP_APIGEN_GEN) $ $(CODE_GENERATOR $(OPENSHIFT_GOIMPORTS)) ## Install tools
.PHONY: tools


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
build: WHAT ?= ./cmd/...
build: clean
	GOOS=$(OS) GOARCH=$(ARCH) go build $(BUILDFLAGS) -ldflags="$(LDFLAGS)" -o bin/ $(WHAT)
.PHONY: build

clean:
	rm -rf bin/*

run-dev-init: build
	bin/bootstrap init --kcp-kubeconfig=${KCP_KUBECONFIG}

run-dev: build
	bin/backend start \
	-v 4 \
	--tls-cert-file=${KCP_REPO_DIR}/127.0.0.1.pem \
	--tls-key-file=${KCP_REPO_DIR}/127.0.0.1.pem \
	--listen-address=127.0.0.1:6443 \
	--oidc-issuer-client-secret=Z2Fyc2lha2FsYmlzdmFuZGVuekWplCg== \
	--oidc-issuer-client-id=kcp-dev \
	--oidc-issuer-url=https://127.0.0.1:5556/dex \
	--oidc-callback-url=https://127.0.0.1:6443/callback \
	--oidc-authorize-url=https://127.0.0.1:6443/authorize \
	--oidc-ca-file=${KCP_REPO_DIR}/127.0.0.1.pem \
	--pretty-name="CorpAAA.com" \
	--namespace-prefix="kube-bind-" \
	--cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
	--cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y= \
	--workspace-path="root:kube-bind" \
	--apiexport-name="kube-bind.io" \
	--kubeconfig=${KCP_KUBECONFIG} \
	--dev-mode=true

crds: $(CONTROLLER_GEN) ## Generate crds
	./hack/update-codegen-crds.sh
.PHONY: crds

codegen: crds
	$(MAKE) imports
.PHONY: codegen

.PHONY: imports
imports: $(OPENSHIFT_GOIMPORTS)
	$(OPENSHIFT_GOIMPORTS) -m github.com/kube-bind/kube-bind/contrib/example-backend-kcp
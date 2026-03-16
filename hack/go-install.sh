#!/usr/bin/env bash

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

# Originally copied from
# https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/c26a68b23e9317323d5d37660fe9d29b3d2ff40c/scripts/go_install.sh
#
# Installs a Go binary either via "go install" or by cloning and building
# from source (for modules with replace directives that block go install).

set -o errexit
set -o nounset
set -o pipefail

if [[ -z "${1:-}" ]]; then
  echo "must provide module as first parameter"
  exit 1
fi

if [[ -z "${2:-}" ]]; then
  echo "must provide binary name as second parameter"
  exit 1
fi

if [[ -z "${3:-}" ]]; then
  echo "must provide version as third parameter"
  exit 1
fi

if [[ -z "${GOBIN:-}" ]]; then
  echo "GOBIN is not set. Must set GOBIN to install the bin in a specified directory."
  exit 1
fi

MODULE="${1}"
BINARY="${2}"
VERSION="${3}"

# Extract the repo URL from the module path (everything before /cmd/...)
# e.g. github.com/kcp-dev/kcp/cmd/kcp -> github.com/kcp-dev/kcp
REPO_URL="${MODULE}"
if [[ "${REPO_URL}" == *"/cmd/"* ]]; then
  REPO_URL="${REPO_URL%%/cmd/*}"
fi
# Relative path within the repo to the binary (e.g. ./cmd/kcp)
CMD_PATH="./${MODULE#"${REPO_URL}"/}"
if [[ "${CMD_PATH}" == "./${MODULE}" ]]; then
  CMD_PATH="."
fi

mkdir -p "${GOBIN}"

# If the versioned binary already exists, just ensure the symlink and exit.
if [[ -f "${GOBIN}/${BINARY}-${VERSION}" ]]; then
  ln -sf "${GOBIN}/${BINARY}-${VERSION}" "${GOBIN}/${BINARY}"
  exit 0
fi

tmp_dir=$(mktemp -d -t goinstall_XXXXXXXXXX)
function clean {
  rm -rf "${tmp_dir}"
}
trap clean EXIT

rm "${GOBIN}/${BINARY}"* > /dev/null 2>&1 || true

cd "${tmp_dir}"

# Try go install first (works when the module has no replace directives).
# The || true prevents errexit from killing the script on failure.
go mod init fake/mod
if go install -tags tools "${MODULE}@${VERSION}" 2>/dev/null; then
  mv "${GOBIN}/${BINARY}" "${GOBIN}/${BINARY}-${VERSION}"
  ln -sf "${GOBIN}/${BINARY}-${VERSION}" "${GOBIN}/${BINARY}"
  exit 0
fi

# Fallback: clone the repo and build from source.
# This handles modules with replace directives in go.mod.
echo "go install failed (module likely has replace directives), cloning and building from source..."
git clone --quiet "https://${REPO_URL}.git" repo
cd repo
git checkout --quiet "${VERSION}"
go build -trimpath -o "${GOBIN}/${BINARY}-${VERSION}" "${CMD_PATH}"
ln -sf "${GOBIN}/${BINARY}-${VERSION}" "${GOBIN}/${BINARY}"

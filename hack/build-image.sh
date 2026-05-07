#!/usr/bin/env bash

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

set -eu

# Inputs (env vars):
#   IMAGE_REPO          registry+namespace, e.g. ghcr.io/kube-bind or kube-bind (required)
#   PLATFORMS           buildx platform list, default: linux/<host arch>
#   IMAGE_TAGS          space-separated tag suffixes, default: $REV
#   PUSH                true|false; default: true if PLATFORMS is multi-arch, else false (--load)
#   LDFLAGS             go ldflags passed as build-arg
#   OUTPUT_DIR          if set, write <component>.metadata.json files (used by image-sign.sh)
#   BUILDX_CACHE_FROM   passed to --cache-from (e.g. type=gha)
#   BUILDX_CACHE_TO     passed to --cache-to (e.g. type=gha,mode=max)
#   IMAGE_LABELS        newline- or space-separated key=value labels (applied to both images)

: "${IMAGE_REPO:?IMAGE_REPO must be set}"
: "${PLATFORMS:=linux/$(go env GOARCH)}"
: "${IMAGE_TAGS:=${REV:?REV must be set when IMAGE_TAGS is unset}}"

if [[ -z "${PUSH:-}" ]]; then
   if [[ "$PLATFORMS" == *","* ]]; then
      PUSH=true
   else
      PUSH=false
   fi
fi

if [[ "$PUSH" == "true" ]]; then
   OUTPUT_FLAG="--push"
else
   OUTPUT_FLAG="--load"
   if [[ "$PLATFORMS" == *","* ]]; then
      echo "PUSH=false with multi-platform PLATFORMS=$PLATFORMS will fail (--load only supports a single platform)." >&2
      exit 1
   fi
fi

command -v docker >/dev/null 2>&1 || { echo "docker not found. Please install Docker"; exit 1; }
docker buildx version >/dev/null 2>&1 || { echo "docker buildx not found. Please enable buildx in Docker"; exit 1; }

# Reuse an existing active builder (e.g. one set up by docker/setup-buildx-action in CI).
# Only create our own if none is active — otherwise we'd lose the GHA cache wiring.
if ! docker buildx inspect >/dev/null 2>&1; then
   docker buildx create --name kube-bind-builder --use >/dev/null
fi
docker buildx inspect --bootstrap >/dev/null 2>&1

cache_args=()
if [[ -n "${BUILDX_CACHE_FROM:-}" ]]; then
   cache_args+=("--cache-from" "$BUILDX_CACHE_FROM")
fi
if [[ -n "${BUILDX_CACHE_TO:-}" ]]; then
   cache_args+=("--cache-to" "$BUILDX_CACHE_TO")
fi

label_args=()
if [[ -n "${IMAGE_LABELS:-}" ]]; then
   while IFS= read -r label; do
      [[ -z "$label" ]] && continue
      label_args+=("--label" "$label")
   done <<< "$(echo "$IMAGE_LABELS" | tr ' ' '\n')"
fi

build_image() {
   local component="$1"
   local dockerfile="$2"

   local tag_args=()
   for t in $IMAGE_TAGS; do
      tag_args+=("-t" "${IMAGE_REPO}/${component}:${t}")
   done

   local metadata_args=()
   if [[ -n "${OUTPUT_DIR:-}" ]]; then
      mkdir -p "$OUTPUT_DIR"
      metadata_args+=("--metadata-file" "${OUTPUT_DIR}/${component}.metadata.json")
   fi

   echo "Building ${component} image (platforms: $PLATFORMS, push: $PUSH)..."
   docker buildx build \
      --platform "$PLATFORMS" \
      --build-arg LDFLAGS="${LDFLAGS:-}" \
      "${tag_args[@]}" \
      "${cache_args[@]}" \
      "${label_args[@]}" \
      "${metadata_args[@]}" \
      -f "$dockerfile" \
      $OUTPUT_FLAG .
}

build_image konnector Dockerfile.konnector
build_image backend Dockerfile

echo "Successfully built images on platforms ${PLATFORMS}"
echo "Tags:"
for t in $IMAGE_TAGS; do
   echo "  ${IMAGE_REPO}/konnector:${t}"
   echo "  ${IMAGE_REPO}/backend:${t}"
done

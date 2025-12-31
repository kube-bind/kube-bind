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

echo "Building images locally with tag $REV for platforms: $PLATFORMS"

command -v docker >/dev/null 2>&1 || { echo "docker not found. Please install Docker"; exit 1; }
docker buildx version >/dev/null 2>&1 || { echo "docker buildx not found. Please enable buildx in Docker"; exit 1; }

# Create buildx builder if it doesn't exist
docker buildx create --name kube-bind-builder --use 2>/dev/null || docker buildx use kube-bind-builder 2>/dev/null || true
docker buildx inspect --bootstrap >/dev/null 2>&1

# Check if building for multiple platforms
if [[ "$PLATFORMS" == *","* ]]; then
   echo "Multi-platform build detected. Images will be pushed to registry instead of loaded locally."
   LOAD_FLAG="--push"
else
   echo "Single platform build. Images will be loaded to local Docker daemon."
   LOAD_FLAG="--load"
fi

echo "Building konnector image..."
docker buildx build \
   --platform $PLATFORMS \
   --build-arg LDFLAGS="$LDFLAGS" \
   -t "$IMAGE_REPO/konnector:$REV" \
   -f Dockerfile.konnector \
   $LOAD_FLAG .

echo "Building backend image..."
docker buildx build \
   --platform $PLATFORMS \
   --build-arg LDFLAGS="$LDFLAGS" \
   -t "$IMAGE_REPO/backend:$REV" \
   -f Dockerfile \
   $LOAD_FLAG .

echo "Successfully built images:"
echo "  $IMAGE_REPO/konnector:$REV ($PLATFORMS)"
echo "  $IMAGE_REPO/backend:$REV ($PLATFORMS)"

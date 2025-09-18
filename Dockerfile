# Copyright 2022 The Kube Bind Authors.
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

FROM golang:1.24.0 AS go-build-env
WORKDIR /app

# Accept build arguments
ARG LDFLAGS

RUN apt-get update && apt-get install -y make jq

# <- COPY go.mod and go.sum files to the workspace
COPY go.mod .
COPY go.sum .

# COPY the source code as the last step
COPY . .

# Build with custom LDFLAGS if provided, otherwise use make build
RUN if [ -n "$LDFLAGS" ]; then \
        echo "Building with LDFLAGS: $LDFLAGS"; \
        go build -ldflags="$LDFLAGS" -o bin/backend ./cmd/backend; \
    else \
        make build; \
    fi

# Use node:lts-alpine for better compatibility and smaller size
FROM node:20.18.0-alpine3.20 AS ui-build-env
WORKDIR /app

# Install build dependencies needed for native modules
RUN apk add --no-cache python3 make g++ 

# Copy package files
COPY ./web/package*.json ./
COPY ./web/.npmrc ./

RUN npm install

# Install dependencies with specific flags to handle optional deps and architecture issues
RUN npm ci --prefer-offline --no-audit --no-fund --no-optional

# Copy the Vue app files
COPY ./web .

# Set environment to avoid native dependency issues
ENV NODE_ENV=production
ENV VITE_BUILD_TARGET=docker

# Building UI with Docker-specific config
RUN npm run build

FROM alpine:3.22.1
RUN apk --update add ca-certificates

COPY --from=go-build-env /app/bin/backend /bin
COPY --from=ui-build-env /app/dist /www



ENTRYPOINT ["/bin/backend"]

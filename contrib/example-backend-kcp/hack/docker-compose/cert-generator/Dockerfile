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

# Stage 1: Build genkey tool
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Set GOBIN to install binaries to /go/bin
ENV GOBIN=/go/bin

# Install genkey tool
RUN go install github.com/mjudeikis/genkey@24d5855234ab9622a9d3ac99dab8277b0d24e5c4

# Stage 2: Create the final image
FROM alpine:latest

RUN apk add --no-cache ca-certificates bash

WORKDIR /cert-generator

# Copy the generate_certs.sh script
COPY generate_certs.sh .

# Make the script executable
RUN chmod +x generate_certs.sh

# Copy the genkey binary from the builder stage
COPY --from=builder /go/bin/genkey /usr/local/bin/genkey

# Volume to share certificates
VOLUME /certs

# Entry point to generate certificates
ENTRYPOINT ["./generate_certs.sh"]
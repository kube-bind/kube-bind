# Stage 1: Build Dex
FROM golang:1.23.3-alpine3.20@sha256:c694a4d291a13a9f9d94933395673494fc2cc9d4777b85df3a7e70b3492d3574 AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make alpine-sdk ca-certificates openssl clang lld

# Clone the Dex repository
RUN git clone https://github.com/mjudeikis/dex.git -b mjudeikis/groups.support /app/dex --dept 1

WORKDIR /app/dex

ENV CGO_ENABLED=1

RUN go mod download

# Build Dex binary
RUN make build

# Stage 2: Create the final image
FROM alpine:latest

RUN apk add --no-cache ca-certificates bash

WORKDIR /app

# Copy Dex binary
COPY --from=builder /app/dex/bin/dex /usr/local/bin/dex

# Create necessary directories
RUN mkdir -p /dex/config /certs /data

# Copy Dex configuration
COPY ./kcp-config.yaml /dex/config/kcp-config.yaml

# Expose Dex port
EXPOSE 5556

# Run Dex
CMD ["dex", "serve", "/dex/config/kcp-config.yaml"]
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
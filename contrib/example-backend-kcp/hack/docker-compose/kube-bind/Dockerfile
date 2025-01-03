# Stage 1: Build Kube-Bind
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Clone the kube-bind repository (replace with the correct repository if different)
COPY / ./

WORKDIR /app/contrib/example-backend-kcp

RUN go mod download

# Build Kube-Bind binary
RUN go build -o kubebind ./cmd/backend/
RUN go build -o kubebind-bootstrap ./cmd/bootstrap/

# Stage 2: Create the final image
FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache ca-certificates bash curl tar

WORKDIR /app

# Copy Kube-Bind binary
COPY --from=builder /app/contrib/example-backend-kcp/kubebind /usr/local/bin/kubebind
COPY --from=builder /app/contrib/example-backend-kcp/kubebind-bootstrap /usr/local/bin/kubebind-bootstrap

ENV PATH="${KREW}/bin:${PATH}"

# Expose Kube-Bind port
EXPOSE 6444

# Set up the alias for kubectl (optional, similar to KCP)
RUN echo "alias k=kubectl" >> /root/.bashrc

# Ensure that bash is the default shell
SHELL ["/bin/bash", "-c"]

# Run Kube-Bind (command is overridden by docker-compose)
CMD ["kubebind"]
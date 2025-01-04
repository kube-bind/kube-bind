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

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/kubectl

# Install krew
ENV KREW=/root/.krew

RUN set -x \
    && apk add --no-cache git \
    && curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/krew-linux_amd64.tar.gz" \
    && tar zxvf krew-linux_amd64.tar.gz \
    && ./krew-linux_amd64 install krew \
    && rm -rf krew-linux_amd64.tar.gz \
    && mv /root/.krew/bin/kubectl-krew /usr/local/bin/kubectl-krew

# Update PATH for krew plugins
ENV PATH="${KREW}/bin:${PATH}"

# Verify installations
RUN kubectl version --client && kubectl krew version

# Add krew plugin index and install desired plugins
RUN kubectl krew index add kcp-dev https://github.com/kcp-dev/krew-index.git \
    && kubectl krew install kcp-dev/kcp \
    && kubectl krew install kcp-dev/ws \
    && kubectl krew install kcp-dev/create-workspace

RUN kubectl krew index add bind https://github.com/kube-bind/krew-index.git \
    && kubectl krew install bind/bind

# (Optional) Pre-install common krew plugins
# RUN kubectl krew install ctx ns

ENV KUBECONFIG=/kcp/config/.kcp/admin.kubeconfig

# Expose Kube-Bind port
EXPOSE 6444

# Set up the alias for kubectl (optional, similar to KCP)
RUN echo "alias k=kubectl" >> /root/.bashrc

# Ensure that bash is the default shell
SHELL ["/bin/bash", "-c"]

# Run Kube-Bind (command is overridden by docker-compose)
CMD ["kubebind"]
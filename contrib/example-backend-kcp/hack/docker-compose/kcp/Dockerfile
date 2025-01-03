# Stage 1: Build KCP
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Clone the KCP repository with a shallow clone
RUN git clone https://github.com/kcp-dev/kcp.git /app/kcp --depth 1

WORKDIR /app/kcp

# Build KCP binary
RUN go build -o kcp ./cmd/kcp

# Stage 2: Create the final image
FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache ca-certificates bash curl tar

WORKDIR /app

# Copy KCP binary
COPY --from=builder /app/kcp/kcp /usr/local/bin/kcp

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

# (Optional) Pre-install common krew plugins
# RUN kubectl krew install ctx ns

ENV KUBECONFIG=/app/.kcp/admin.kubeconfig 

# Set up the alias for kubectl
RUN echo "alias k=kubectl" >> /root/.bashrc

# Ensure that bash is the default shell
SHELL ["/bin/bash", "-c"]

# Expose KCP port
EXPOSE 6443

# Run KCP (command is overridden by docker-compose)
CMD ["kcp"]

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

# This script ensures that the generated client code checked into git is up-to-date
# with the generator. If it is not, re-generate the configuration to update it.

set -o errexit
set -o nounset
set -o pipefail


export BACKEND_HOST_IP=192.168.0.34

cat << EOF_BackendClusterDefinition | kind create cluster --config=-
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
name: backend
nodes:
- role: control-plane
  extraPortMappings:
  # MangoDB export endpoint
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  # DEX endpoint
  - containerPort: 30556
    hostPort: 5556
    protocol: TCP
EOF_BackendClusterDefinition

helm repo add jetstack https://charts.jetstack.io
helm install \
    --create-namespace \
    --namespace pki \
    --version v1.16.2 \
    --set crds.enabled=true \
    cert-manager jetstack/cert-manager


helm repo add dex https://charts.dexidp.io

cat << EOF_DEXDeploymentConfig |
config:
    staticClients:
      - id: kube-bind
        redirectURIs:
          - 'http://${BACKEND_HOST_IP}:8080/callback'
        name: 'Kube Bind'
        secret: ZXhhbXBsZS1hcHAtc2VjcmV0

    issuer: http://${BACKEND_HOST_IP}:5556/dex

    storage:
      type: kubernetes
      config:
        inCluster: true

    web:
      http: 127.0.0.1:5556

    telemetry:
      http: 127.0.0.1:5558

    grpc:
      addr: 127.0.0.1:5557

    connectors:
      - type: mockCallback
        id: mock
        name: Example

    enablePasswordDB: true
    staticPasswords:
      - email: "admin@example.com"
        hash: "\$2a\$10\$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
        username: "admin"
        userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
EOF_DEXDeploymentConfig

helm install \
    --create-namespace \
    --namespace idp \
    --set service.type=NodePort \
    --set service.ports.http.nodePort=30556 \
    dex dex/dex \
    -f -

kubectl apply -f ../../deploy/crd
kubectl apply -f ../../test/e2e/bind/fixtures/provider/crd-mangodb.yaml
kubectl create namespace backend
# This is the address that will be used when generating kubeconfigs the App cluster,
# and so we need to be able to reach it from outside.
export BACKEND_KUBE_API_EXTERNAL_ADDRESS="$(kubectl config view --minify -o json | jq '.clusters[0].cluster.server' -r)"
# For demo example let's just bind "cluster-admin" ClusterRole to backend's "default" ServiceAccount.
kubectl create clusterrolebinding backend-admin --clusterrole cluster-admin --serviceaccount backend:default
# Create a new Deployment for the MangoDB backend.
kubectl --namespace backend \
    create deployment mangodb \
    --image ghcr.io/kube-bind/example-backend:v0.4.6 \
    --port 8080 \
    -- /ko-app/example-backend \
        --listen-address 0.0.0.0:8080 \
        --external-address "${BACKEND_KUBE_API_EXTERNAL_ADDRESS}" \
        --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
        --oidc-issuer-client-id=kube-bind \
        --oidc-issuer-url=http://${BACKEND_HOST_IP}:5556/dex \
        --oidc-callback-url=http://${BACKEND_HOST_IP}:8080/callback \
        --pretty-name="BigCorp.com" \
        --namespace-prefix="kube-bind-" \
        --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
        --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y=

# Expose mangodb's container port 8080 as a NodePort at 30080. We've already configured
# Kind to expose 30800 at host's 8080.
kubectl --namespace backend \
    create service nodeport mangodb \
    --tcp 8080 \
    --node-port 30080

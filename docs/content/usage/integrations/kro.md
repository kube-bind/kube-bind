---
title: kro
description: |
    Guide on integrating kube-bind with kro and Envoy Gateway for serving LoadBalancer as a Service.
weight: 30
---

# kro Integration (providing LoadBalancer as a Service)

This guide demonstrates how to use [kro](https://kro.run/) and [Envoy Gateway](https://gateway.envoyproxy.io/) to offer a "LoadBalancer as a Service" API.
Consumers create a simple `LoadBalancer` object, and the provider automatically provisions an Envoy Gateway and HTTPRoute to expose the service between two kind clusters.

This example includes support for syncing **custom configuration (via ConfigMaps or Secrets)** from consumer clusters to the provider.

## Prerequisites

*   **Provider Cluster:** Runs kro, Envoy Gateway, and the kube-bind backend.
*   **Consumer Cluster:** Runs the kube-bind konnector.

## Setup on Provider Cluster

1.  **Install Envoy Gateway.**
    This handles the actual traffic routing.

```bash
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.6.1 -n envoy-gateway-system --create-namespace
```

    Install GatewayClass to be managed by Envoy Gateway.

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
```

2. **Setup backend application service.**

  Fetch the consumer cluster node IP.
```bash
export CONSUMER_NODE_IP=$(KUBECONFIG=kind-consumer.kubeconfig kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
```

  Setup the provider cluster service to allow connection to the backend application in consumer cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: backend
  namespace: default
spec:
  clusterIP: None
  ports:
    - port: 30080
      targetPort: 30080
      protocol: TCP
      name: http
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: backend
  namespace: default
  labels:
    kubernetes.io/service-name: backend
addressType: IPv4
ports:
  - name: http
    port: 30080
    protocol: TCP
endpoints:
  - addresses:
      - ${CONSUMER_NODE_IP}
    conditions:
      ready: true
EOF
```

2.  **Install kro.**
    kro allows you to define custom APIs (ResourceGroups) and map them to underlying resources.

```bash
helm install kro oci://registry.k8s.io/kro/charts/kro \
--namespace kro-system \
--create-namespace
```

3.  **Define the `LoadBalancer` ResourceGroup.**
    Create a kro `ResourceGraphDefinition` that defines the API `loadbalancers.networking.kro.run`.

    This definition includes:
    *   **Configuration:** References a ConfigMap for custom routing rules (e.g., adding headers). The same way user could reference a Secret with Certificate to setup TLS.

```yaml
kubectl apply -f - <<'EOF'
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: loadbalancers
spec:
  schema:
    apiVersion: v1alpha1
    kind: LoadBalancer
    group: networking.kro.run
    spec:
      domain: string
      configMapRef: string
      targetService: string
      targetServiceNamespace: string
      targetPort: integer | default=8080
    status:
      address: string
  resources:
    - id: configmap
      externalRef:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ${schema.spec.configMapRef}
          namespace: ${schema.metadata.namespace}
    - id: referencegrant
      template:
        apiVersion: gateway.networking.k8s.io/v1beta1
        kind: ReferenceGrant
        metadata:
          name: ${schema.metadata.name}-grant
          namespace: ${schema.spec.targetServiceNamespace}
        spec:
          from:
            - group: gateway.networking.k8s.io
              kind: HTTPRoute
              namespace: ${schema.metadata.namespace}
          to:
            - group: ""
              kind: Service
    - id: gateway
      template:
        apiVersion: gateway.networking.k8s.io/v1
        kind: Gateway
        metadata:
          name: ${schema.metadata.name}-gw
          namespace: ${schema.metadata.namespace}
        spec:
          gatewayClassName: eg
          listeners:
            - name: http
              port: 80
              protocol: HTTP
              hostname: ${schema.spec.domain}
    - id: route
      template:
        apiVersion: gateway.networking.k8s.io/v1
        kind: HTTPRoute
        metadata:
          name: ${schema.metadata.name}-route
          namespace: ${schema.metadata.namespace}
        spec:
          parentRefs:
            - name: ${schema.metadata.name}-gw
          hostnames:
            - ${schema.spec.domain}
          rules:
            - backendRefs:
                - name: ${schema.spec.targetService}
                  namespace: ${schema.spec.targetServiceNamespace}
                  port: ${schema.spec.targetPort}
              filters:
                - type: RequestHeaderModifier
                  requestHeaderModifier:
                    add:
                      - name: X-Custom-Message
                        value: ${configmap.?data["custom-header"]}
EOF
```

4. **Add kube-bind export label** to loadbalancer CRD.

```bash
kubectl label crd loadbalancers.networking.kro.run kube-bind.io/exported=true --overwrite
```

5.  **Export the LoadBalancer API using kube-bind.**
    Create an `APIServiceExportTemplate`. Crucially, we add **PermissionClaims** to allow the provider to read the ConfigMaps that the consumer will create and reference.

```yaml
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportTemplate
metadata:
    name: loadbalancer
spec:
  resources:
  - group: networking.kro.run
    resource: loadbalancers
    versions:
    - v1alpha1
  permissionClaims:
  - group: ""
    resource: configmaps
    selector:
      references:
        - resource: loadbalancers
          group: networking.kro.run
          jsonPath:
            name: 'spec.configMapRef'
  scope: Namespaced
EOF
```

## Consumer Usage

1. **Login to kube-bind and request a binding to the exported loadbalancer API.**

    ```bash
    kubectl bind login https://kube-bind.example.com
    # Authenticate and select the loadbalancer export
    kubectl bind
    ```


2. **Wait for the binding to be established.** Once the binding is active, you can create `LoadBalancer` resources in your consumer cluster, and you will get `LoadBalancer` objects synced from the provider cluster.

```bash
kubectl bind
🌐 Opening kube-bind UI in your browser...
    https://kube-bind.example.com?redirect_url=....
Browser opened successfully
Waiting for binding completion from UI...
   (Press Ctrl+C to cancel)
Binding completed successfully!
🔒 Updated secret kube-bind/kubeconfig-zdrdn for host https://kube-bind.example.com, namespace kube-bind-wz2g5
🚀 Deploying konnector v0.6.0 to namespace kube-bind with custom image "ghcr.io/kube-bind/konnector:v0.6.0".
✅ Created APIServiceBinding loadbalancer for 1 resources
Created 1 APIServiceBinding(s):
  - loadbalancer
Resources bound successfully!
```

3.  **Create Dependencies.**
    The consumer creates the ConfigMap that will be referenced by the LoadBalancer.

```yaml
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
    name: my-lb-config
    namespace: default
data:
    custom-header: "hello-kube-bind"
EOF
```
  Deploy backend application into consumer cluster.

```yaml
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: backend
---
apiVersion: v1
kind: Service
metadata:
  name: backend
  labels:
    app: backend
    service: backend
spec:
  type: NodePort
  ports:
  - name: http
    port: 3000
    targetPort: 3000
    nodePort: 30080
  selector:
    app: backend
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backend
      version: v1
  template:
    metadata:
      labels:
        app: backend
        version: v1
    spec:
      serviceAccountName: backend
      containers:
      - image: gcr.io/k8s-staging-gateway-api/echo-basic:v20231214-v1.0.0-140-gf544a46e
        imagePullPolicy: IfNotPresent
        name: backend
        ports:
        - containerPort: 3000
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
EOF
```

3.  **Create a LoadBalancer.**
    Reference the ConfigMap created above.

```yaml
kubectl apply -f - <<EOF
apiVersion: networking.kro.run/v1alpha1
kind: LoadBalancer
metadata:
    name: secure-lb
    namespace: default
spec:
    domain: "www.example.com"
    configMapRef: "my-lb-config"
    targetService: "backend"
    targetServiceNamespace: "default"
    targetPort: 30080
EOF
```

4.  **Observe the provisioning.**

*   **Provider Side:** kube-bind syncs the ConfigMap back to the provider namespace. kro creates the Gateway, Route and ReferenceGrant, and Envoy Gateway provisions the load balancer.
*   **Consumer Side:** The status is updated with the provider status.

Check loadbalancer in the consumer cluster.
```bash
kubectl get loadbalancer secure-lb

NAMESPACE   NAME        STATE    READY   AGE
default     secure-lb   ACTIVE   True    14s
```

Check loadbalancer in the provider cluster and if the configmap is synced.

```bash
kubectl get loadbalancers.networking.kro.run -A

NAMESPACE                 NAME        STATE    READY   AGE
kube-bind-wz2g5-default   secure-lb   ACTIVE   True    15s
```

```bash
kubectl get cm -n kube-bind-wz2g5-default

NAME               DATA   AGE
kube-root-ca.crt   1      3h14m
my-lb-config       1      15s
```

5.  **Appendix.**
*   Test the connection with provisioned load balancer and verify that `hello-kube-bind` header was added from the ConfigMap.

!!! note
    For this to work end-to-end, the consumer's service (`backend`) must be reachable from the provider cluster (e.g., via multi-cluster networking) or synced to the provider cluster.


```bash
curl --verbose --header "Host: www.example.com" http://${PROVIDER_CLUSTER_LB_IP}/headers
```

*   Trying 172.18.0.200:80...
* Connected to 172.18.0.200 (172.18.0.200) port 80
> GET /headers HTTP/1.1
> Host: www.example.com
> User-Agent: curl/8.7.1
> Accept: */*
>
* Request completely sent off
< HTTP/1.1 200 OK
< content-type: application/json
< x-content-type-options: nosniff
< date: Fri, 19 Dec 2025 19:50:35 GMT
< content-length: 521
< x-response-message: hello-kube-bind
<
{
 "path": "/headers",
 "host": "www.example.com",
 "method": "GET",
 "proto": "HTTP/1.1",
 "headers": {
  "Accept": [
   "*/*"
  ],
  "User-Agent": [
   "curl/8.7.1"
  ],
  "X-Custom-Message": [
   "hello-kube-bind"
  ],
  "X-Envoy-External-Address": [
   "172.18.0.2"
  ],
  "X-Forwarded-For": [
   "172.18.0.2"
  ],
  "X-Forwarded-Proto": [
   "http"
  ],
  "X-Request-Id": [
   "8c23f131-a328-485f-b7e1-2e6b20362af1"
  ]
 },
 "namespace": "default",
 "ingress": "",
 "service": "",
 "pod": "backend-77d4d5968-glxtp"
* Connection #0 to host 172.18.0.200 left intact
}

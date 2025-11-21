---
description: >
  How to test changes made to kube-bind in your development environment.
weight: 30
title: Testing Changes
---

# Testing code changes

When making changes to kube-bind, it's important to test them in a realistic multi-cluster environment.

Follow [development setup instructions](../developers/development-setup/) to set up your development environment using kcp.
kcp allows you to simulate multiple clusters using logical clusters.


# Testing helm chart changes

By default, in helm chart, the backend component does not have TLS enabled, and the embedded OIDC server is not used.
To test changes related to TLS or OIDC, you need to enable them explicitly by setting the appropriate Helm values.

To test basic Helm-install flow you will need GatewayAPI enabled kubernetes cluster with cert-manager installed.
By default it will use TLS termination at the Gateway level.


```bash
# Use a specific development version:
# VERSION=0.0.0-9fd9281e661c0d9a426a941111d3d8b08019ebc1
```

And run full helm install command with additional parameters:
```bash
helm upgrade --install \
      --namespace kube-bind \
      --create-namespace \
      --set certManager.enabled=true \
      --set certManager.clusterIssuer=letsencrypt-prod \
      --set backend.oidc.issuerUrl=https://auth.genericcontrolplane.io \
      --set backend.oidc.clientId=platform-mesh \
      --set backend.oidc.clientSecret=Z2Fyc2lha2FsYmlzdmFuZGVuekWplCg== \
      --set backend.oidc.callbackUrl=https://kube-bind.genericcontrolplane.io/api/callback \
      --set gatewayApi.enabled=true \
      --set gatewayApi.gateway.className=nginx \
      --set gatewayApi.gateway.httpPort=80 \
      --set gatewayApi.gateway.httpsPort=443 \
      --set 'gatewayApi.gateway.tls.certificateRefs[0].name=backend-tls-cert' \
      --set 'gatewayApi.route.hostnames[0]=kube-bind.genericcontrolplane.io' \
      --set gatewayApi.route.path=/ \
      --set gatewayApi.route.pathType=PathPrefix \
      --set image.tag=${VERSION} \
      kube-bind \
      ./deploy/charts/backend
```

After the deployment at minimum url should be accessible:

```bash
 curl https://kube-bind.genericcontrolplane.io
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Kube Bind</title>
    <script type="module" crossorigin src="./assets/index.41dda553.js"></script>
    <link rel="stylesheet" href="./assets/index.952308d6.css">
  </head>
  <body>
    <div id="app"></div>
    
  </body>
</html>%                                                           
```


# Local dev environment testing

If you changed helm charts and neet to test them in local development environment you can do the following:

```bash
 ./bin/kubectl-bind dev create --chart-path ./deploy/charts/backend       
```
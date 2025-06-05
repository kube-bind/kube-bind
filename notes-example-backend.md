# example-backend

## Controllers/Reconcilers

example-backend reconciles following object types:
* ClusterBinding
* APIServiceExport
* APIServiceExportRequest
* APIServiceNamespace

### ClusterBinding

Impl:
[`./contrib/example-backend/controllers/clusterbinding/clusterbinding_controller.go`]
[`./contrib/example-backend/controllers/clusterbinding/clusterbinding_reconcile.go`]

Types:
[`./sdk/apis/kubebind/v1alpha1/clusterbinding_types.go`]
[`./sdk/apis/kubebind/v1alpha1/apiserviceexport_types.go`]

Watches:
* `kubebindv1alpha1.ClusterBinding`
  - Key:
* `kubebindv1alpha1.APIServiceExport`

Ensures that:
* ClusterBinding conditions are set correctly in case of error states on the obj status
* RoleBindings exist or need to be created
  - Meta: name `kube-binder` in `clusterBinding.Namespace` namespace
  - Subject: ServiceAccount `kube-binder` in `clusterBinding.Namespace`
  - RoleRef: ClusterRole `kube-binder`
* ClusterRoles exist or need to be created
  - Name: `kube-binder-{clusterBinding.Namespace}`
  - Owner: Kind `v1/Namespace` name `{clusterBinding.Namespace}`
  - One rule for one APIServiceExport:
    - APIGroup: `{APIServiceExport.Spec.Group}`
    - Resources: `{APIServiceExport.Spec.Names.Plural}`
    - Verbs: `"get", "list", "watch", "update", "patch", "delete", "create"`
* ClusterRoleBindings exist or need to be created
  - Meta: `kube-binder-{clusterBinding.Namespace}`
  - Owner: Kind `v1/Namespace` name `{clusterBinding.Namespace}`
  - Subject: `ServiceAccount` name `kube-binder` in `{clusterBinding.Namespace}` namespace
  - RoleRef: ClusterRole `kube-binder-{clusterBinding.Namespace}`

### APIServiceExport

Impl:
[`./contrib/example-backend/controllers/serviceexport/serviceexport_controller.go`]
[`./contrib/example-backend/controllers/serviceexport/serviceexport_reconcile.go`]

Types:
[`./sdk/apis/kubebind/v1alpha1/apiserviceexport_types.go`]

Watches:
* `kubebindv1alpha1.APIServiceExport`
* `apiextensionsv1.CustomResourceDefinition`

Ensures that:
* APIServiceExport objects are in sync with their respective CRDs.
  - Gets CRD hash (via `hash(toAPIServiceExport(crds[export.Name]))`)
  - If the export's hash differs from CRD's hash, the export is updated with the updated CRD
  - The hash is cached in APIServiceExport's annotation `kube-bind.io/source-spec-hash`
  - If CRD doesn't exist, the APIServiceExport is deleted too.

### APIServiceExportRequest

Impl:
[`./contrib/example-backend/controllers/serviceexportrequest/serviceexportrequest_controller.go`]
[`./contrib/example-backend/controllers/serviceexportrequest/serviceexportrequest_reconcile.go`]

Types:
[`./sdk/apis/kubebind/v1alpha1/apiserviceexportrequest_types.go`]
[`./sdk/apis/kubebind/v1alpha1/apiserviceexport_types.go`]

Watches:
* `kubebindv1alpha1.APIServiceExportRequest`
* `kubebindv1alpha1.APIServiceExport`
* `apiextensionsv1.CustomResourceDefinition`

Ensures that:
* APIServiceExportRequest are fulfilled to create APIServiceExport with the respective CRDs:
  - If an APIServiceExportRequest status is in `kubebindv1alpha1.APIServiceExportRequestPhasePending=` phase:
    - For each resource in `APIServiceExportRequestSpec.Resources []APIServiceExportRequestResource`
    - `name = "{res.Resource}.{res.Group}"`
    - Gets CRD with `name` ; if not found, mark Status with CRDNotFound error
    - Gets APIServiceExport in `req.Namespace` ns with name `name` ; if found, continue with next resource ; if other errors, return err
    - Convert the CRD to APIServiceExportCRDSpec, calculate the CRD's hash
    - Create a new APIServiceExport kAPI object with the spec and hash annotation
    - If the creation took more than 1m, fail (~ why?)
    - return success
* APIServiceExportRequest is deleted when it exists for longer than 10m (~ better request clean up handling?)

### APIServiceNamespace

Impl:
[`./contrib/example-backend/controllers/servicenamespace/servicenamespace_controller.go`]
[`./contrib/example-backend/controllers/servicenamespace/servicenamespace_reconcile.go`]

Types:
[`./sdk/apis/kubebind/v1alpha1/apiservicenamespace_types.go`]
[`./sdk/apis/kubebind/v1alpha1/clusterbinding_types.go`]
[`./sdk/apis/kubebind/v1alpha1/apiserviceexport_types.go`]

Watches:
* `kubebindv1alpha1.APIServiceNamespace`
* `kubebindv1alpha1.ClusterBinding`
  - Gets ClusterBinding's namespace, and enqueues all APIServiceNamespaces that are indexed in it.
* `kubebindv1alpha1.APIServiceExport`
  - Gets APIServiceExport's namespace, and enqueues all APIServiceNamespaces that are indexed in it.
* `corev1.Namespace`
  - Enqueues all APIServiceNamespaces that are indexed in this namespace.

Ensures that:
* corev1/Namespace with name `{sns.Namespace}-{sns.Name}` exists, or is created:
  - Populate the Namespace object with name, APIServiceNamespaceAnnotationKey annotation with `{sns.Namespace}/{sns.Name}` value
* RoleBinding for the ServiceAccount exists or is created:
  - Name: `kube-binder`
  - Namespace: this namespace
  - Subject: ServiceAccount in `sns.Namespace` with name `kuberesources.ServiceAccountName=kube-binder`
  - RoleRef: ClusterRole with name `kube-binder-{sns.Namespace}`

## kube-bind -- example-backend flow

### 1. `kubectl bind <host>/export`

* Client makes a GET request to `<host>/export`
* Select an authorization provider; only OAuth2 code grant is supported for the moment
* OAuth2CodeGrant is given an auth URL to continue, `http://{http.Request.Host}/authorize`
* The poulated `kubebindv1alpha1.BindingProvider` struct is JSON-encoded and sent as a response

### 2. Redirect client to `/authorize`

* The client continues to the auth URL
* Defines scopes, redirect URL (`/callback`), session ID, cluster ID in `AuthCode` struct
* The struct is JSON-encoded
* Response header is set to redirect to the "redirect_url", and sent back to the client

### 3. Redirect to `/callback`

* Process the OAuth2 code and state, parse JWT, create session and its cookie
* Response header is set to redirect to `/resources`, and sent back to the client

### 4. Redirect client to `/resources`

* Lists CRDs that have `kube-bind.io/exported: "true"` label
* The CRDs are then filetered so that `h.scope == kubebindv1alpha1.ClusterScope || crd.Spec.Scope == apiextensionsv1.NamespaceScoped`
* The slice is marshalled to JSON and to the client sent as response

### 5. (UI) Client continues to `/bind`

TODO

### HTTP handler

`./contrib/example-backend/http/handler.go`

Adds handlers for routes:
* `/export`
  - Called from `kubectl bind`
  - Populates `kubebindv1alpha1.BindingProvider` object with OAuth2 code grant flow
  - Generates URL where to continue authorization: `http://{http.Request.Host}/authorize`
  - Writes the JSON-formatted object to response
* `/resources`
  - Lists CRDs that have `kube-bind.io/exported: "true"` label
  - The CRDs are then filetered so that `h.scope == kubebindv1alpha1.ClusterScope || crd.Spec.Scope == apiextensionsv1.NamespaceScoped`
  - The slice is marshalled to JSON and sent as response
* `/bind`
  - Called from the UI
  - Decodes and decrypts the cookie to get the session state
  - Gets kubeconfig from `kfg, err := h.kubeManager.HandleResources(r.Context(), idToken.Subject+"#"+state.ClusterID, resource, group)`
  - JSON-Encodes `kubebindv1alpha1.APIServiceExportRequestResponse` for `group` and `resource`
  - JSON-Encodes `kubebindv1alpha1.BindingResponse` with kubeconfig and `APIServiceExportRequestResponse` JSON from the step above
  - URLEncode `BindingResponse`
* `/authorize`
  - OAuth2 code flow authorization...
* `/callback`
  - OAuth2 code flow authorization and cookie set up


export interface BindingTemplate {
  name: string
}

export interface BindableResourcesRequest {
  metadata: {
    name: string
  }
  templateRef: BindingTemplate
}

export interface APIServiceExportRequestResponse {
  apiVersion: string
  kind: string
  metadata: {
    name: string
  }
  spec: {
    resources: any[]
    permissionClaims: any[]
    namespaces: any[]
  }
}

export interface BindingResponseAuthentication {
  oauth2CodeGrant: {
    sessionID: string
    id: string
  }
}

export interface BindingResponse {
  apiVersion: string
  kind: string
  authentication: BindingResponseAuthentication
  kubeconfig: string
  requests: any[]
}

export interface BindingResult {
  success: boolean
  data?: BindingResponse
  message?: string
  error?: string
}
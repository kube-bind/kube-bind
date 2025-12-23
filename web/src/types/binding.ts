export interface BindingTemplate {
  name: string
}

export interface ClusterIdentity {
  identity: string
  name?: string
}

export interface BindableResourcesRequest {
  metadata: {
    name: string
  }
  templateRef: BindingTemplate
  clusterIdentity: ClusterIdentity
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

// Structured error response from backend
export interface KubeBindError {
  kind: 'Error'
  apiVersion: string
  message: string
  code: string
  details?: string
}

// Common error codes from backend
export const ErrorCodes = {
  AUTHENTICATION_FAILED: 'AUTHENTICATION_FAILED',
  AUTHORIZATION_FAILED: 'AUTHORIZATION_FAILED',
  RESOURCE_NOT_FOUND: 'RESOURCE_NOT_FOUND',
  INTERNAL_ERROR: 'INTERNAL_ERROR',
  BAD_REQUEST: 'BAD_REQUEST',
  CLUSTER_CONNECTION_FAILED: 'CLUSTER_CONNECTION_FAILED'
} as const

export type ErrorCode = typeof ErrorCodes[keyof typeof ErrorCodes]

// Helper function to check if an error response is a structured KubeBindError
export function isKubeBindError(error: any): error is KubeBindError {
  return error && typeof error === 'object' && error.kind === 'Error' && error.apiVersion && error.message && error.code
}
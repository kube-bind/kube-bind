import axios from 'axios'

export interface SessionInfo {
  sessionId: string
  clusterId: string
  isAuthenticated: boolean
}

export interface BindableResource {
  name: string
  kind: string
  scope: string
  apiVersion: string
  group: string
  resource: string
  sessionID: string
}

export interface PermissionClaim {
  // Add permission claim properties as needed
  [key: string]: any
}

export interface BindableResourcesRequest {
  apiVersion: string
  kind: string
  resources: BindableResource[]
  permissionClaims?: PermissionClaim[]
}

class AuthService {
  private sessionInfo: SessionInfo | null = null

  async checkAuthentication(): Promise<boolean> {
    try {
      const sessionCookie = this.getSessionCookie()
      if (!sessionCookie) {
        return false
      }
      
      return true
    } catch (error) {
      console.error('Auth check failed:', error)
      return false
    }
  }

  login(clusterId: string = '', redirectPort: string = '3000'): void {
    const sessionId = this.generateSessionId()
    const redirectUrl = `${window.location.origin}/api/callback`
    
    // Use cluster-aware endpoint if clusterId is provided
    const authPath = clusterId ? `/api/clusters/${clusterId}/authorize` : '/api/authorize'
    const authUrl = new URL(authPath, window.location.origin)
    authUrl.searchParams.set('s', sessionId)
    authUrl.searchParams.set('c', clusterId || 'default')
    authUrl.searchParams.set('u', redirectUrl)
    authUrl.searchParams.set('p', redirectPort)

    this.sessionInfo = {
      sessionId,
      clusterId: clusterId || 'default',
      isAuthenticated: false
    }

    window.location.href = authUrl.toString()
  }

  logout(): void {
    this.sessionInfo = null
    this.clearSessionCookie()
  }

  getSessionInfo(): SessionInfo | null {
    return this.sessionInfo
  }

  private generateSessionId(): string {
    return Math.random().toString(36).substring(2) + Date.now().toString(36)
  }

  private getSessionCookie(): string | null {
    const cookies = document.cookie.split(';')
    for (let cookie of cookies) {
      const [name, value] = cookie.trim().split('=')
      if (name.startsWith('kube-bind')) {
        return value
      }
    }
    return null
  }

  private clearSessionCookie(): void {
    const cookies = document.cookie.split(';')
    for (let cookie of cookies) {
      const [name] = cookie.trim().split('=')
      if (name.startsWith('kube-bind')) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
      }
    }
  }

  async getResources(clusterId: string = ''): Promise<any[]> {
    try {
      const sessionCookie = this.getSessionCookie()
      if (!sessionCookie) {
        throw new Error('No session found')
      }

      // Use cluster-aware endpoint if clusterId is provided
      const resourcesPath = clusterId ? `/api/clusters/${clusterId}/resources` : '/api/resources'
      const response = await axios.get(`${resourcesPath}?s=${sessionCookie}`)
      return response.data
    } catch (error) {
      console.error('Failed to fetch resources:', error)
      throw error
    }
  }

  async bindResource(group: string, resource: string, version: string, clusterId: string = ''): Promise<any> {
    try {
      const sessionCookie = this.getSessionCookie()
      if (!sessionCookie) {
        throw new Error('No session found')
      }

      return this.bindResourceWithSession(group, resource, version, clusterId, sessionCookie)
    } catch (error) {
      console.error('Failed to bind resource:', error)
      throw error
    }
  }

  async getResourcesWithSession(clusterId: string = '', sessionId: string): Promise<any> {
    try {
      // Use cluster-aware endpoint if clusterId is provided
      const resourcesPath = clusterId ? `/api/clusters/${clusterId}/resources` : '/api/resources'
      const fullUrl = `${resourcesPath}?s=${sessionId}`
      
      console.log('🌐 Making API request to:', fullUrl)
      console.log('🔑 Session ID:', sessionId)
      console.log('🏷️ Cluster ID:', clusterId || 'none (single cluster)')
      
      const response = await axios.get(fullUrl)
      
      console.log('✅ API Response Status:', response.status)
      console.log('📄 Response Headers:', response.headers)
      console.log('📦 Response Data:', response.data)
      
      return response.data
    } catch (error: any) {
      console.error('❌ Failed to fetch resources with session:', error)
      if (error.response) {
        console.error('📄 Error Response Status:', error.response.status)
        console.error('📄 Error Response Data:', error.response.data)
        console.error('📄 Error Response Headers:', error.response.headers)
      }
      throw error
    }
  }

  async bindResourceWithSession(group: string, resource: string, version: string, clusterId: string = '', sessionId: string, scope: string = 'Namespaced', kind: string = '', name: string = ''): Promise<any> {
    try {
      console.log('🔗 Binding resource with POST request')
      console.log('📋 Resource details:', { group, resource, version, clusterId, sessionId })
      
      // Use cluster-aware endpoint if clusterId is provided
      const bindPath = clusterId ? `/api/clusters/${clusterId}/bind` : '/api/bind'
      const bindUrl = `${bindPath}?s=${sessionId}`
      
      console.log('🌐 POST request to:', bindUrl)
      
      // Create the BindableResourcesRequest payload
      const requestPayload: BindableResourcesRequest = {
        apiVersion: 'kubebind.io/v1alpha2',
        kind: 'BindableResourcesRequest',
        resources: [{
          name: name || `${resource}.${group || 'core'}`,
          kind: kind || resource,
          scope: scope,
          apiVersion: version,
          group: group || '',
          resource: resource,
          sessionID: sessionId
        }],
        permissionClaims: []
      }
      
      console.log('📦 Request payload:', requestPayload)
      
      const response = await axios.post(bindUrl, requestPayload, {
        headers: {
          'Content-Type': 'application/json'
        }
      })
      
      console.log('✅ Bind response status:', response.status)
      console.log('📦 Bind response data:', response.data)
      
      return response.data
    } catch (error: any) {
      console.error('❌ Failed to bind resource with session:', error)
      if (error.response) {
        console.error('📄 Error Response Status:', error.response.status)
        console.error('📄 Error Response Data:', error.response.data)
      }
      throw error
    }
  }

  async getExports(clusterId: string = ''): Promise<any> {
    try {
      // Use cluster-aware endpoint if clusterId is provided
      const exportsPath = clusterId ? `/api/clusters/${clusterId}/exports` : '/api/exports'
      const response = await axios.get(exportsPath)
      return response.data
    } catch (error) {
      console.error('Failed to fetch exports:', error)
      throw error
    }
  }
}

export const authService = new AuthService()
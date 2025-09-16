import axios from 'axios'

export interface SessionInfo {
  sessionId: string
  clusterId: string
  isAuthenticated: boolean
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

  async bindResource(group: string, resource: string, version: string, clusterId: string = ''): Promise<void> {
    try {
      const sessionCookie = this.getSessionCookie()
      if (!sessionCookie) {
        throw new Error('No session found')
      }

      // Use cluster-aware endpoint if clusterId is provided
      const bindPath = clusterId ? `/api/clusters/${clusterId}/bind` : '/api/bind'
      const bindUrl = `${bindPath}?group=${group}&resource=${resource}&version=${version}&s=${sessionCookie}`
      window.location.href = bindUrl
    } catch (error) {
      console.error('Failed to bind resource:', error)
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
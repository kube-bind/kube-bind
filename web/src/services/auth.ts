
export interface AuthStatus {
  isAuthenticated: boolean
  sessionId?: string
  clusterId?: string
}

class AuthService {
  async isAuthenticated(): Promise<boolean> {
    try {
      // Since cookies are HTTP-only, we need to check authentication by making an API call
      const urlParams = new URLSearchParams(window.location.search)
      const clusterId = urlParams.get('cluster_id') || ''
      
      // Make a simple API call to check if we're authenticated
      const authCheckUrl = clusterId ? `/ping?cluster_id=${clusterId}` : '/ping'
      
      const response = await fetch(`/api${authCheckUrl}`, {
        method: 'GET',
        credentials: 'include' // Include cookies
      })
      
      const isAuth = response.status === 200
      console.log('Auth check:', { clusterId, status: response.status, isAuth })
      
      return isAuth
    } catch (error) {
      console.error('Auth check error:', error)
      return false
    }
  }

  async initiateAuth(
    sessionId: string, 
    clusterId: string,
    clientSideRedirectUrl?: string,
  ): Promise<void> {
    const authUrl = `/api/authorize`
    
    const redirect_url = window.location.origin + window.location.pathname
    
    const params = new URLSearchParams({
      session_id: sessionId,
      redirect_url: redirect_url,
      cluster_id: clusterId,
      client_type: 'ui' // Use UI type to get cookies
    })

    if (clientSideRedirectUrl) {
      params.set('client_side_redirect_url', clientSideRedirectUrl)
    }

    window.location.href = `${authUrl}?${params}`
  }

  async logout(): Promise<void> {
    try {
      // Call backend logout endpoint to clear HttpOnly cookies
      const urlParams = new URLSearchParams(window.location.search)
      const clusterId = urlParams.get('cluster_id') || ''
      
      // Build logout URL with cluster_id parameter if present
      const logoutUrl = clusterId 
        ? `/api/logout?cluster_id=${encodeURIComponent(clusterId)}`
        : '/api/logout'
      
      // Make POST request to logout endpoint
      const response = await fetch(logoutUrl, {
        method: 'POST',
        credentials: 'include' // Include cookies in the request
      })
      
      console.log('Logout response:', { status: response.status, clusterId })
      
      // Redirect to clear any cached state regardless of response
      window.location.href = window.location.origin + window.location.pathname
    } catch (error) {
      console.error('Logout error:', error)
      // Still try to reload even if there was an error
      window.location.reload()
    }
  }

  getSessionCookieName(): string | null {
    // Since cookies are HTTP-only, we can't read them from JavaScript
    // The session parameter will be added automatically by the HTTP interceptor
    // based on the current cluster_id in the URL
    const urlParams = new URLSearchParams(window.location.search)
    const clusterId = urlParams.get('cluster_id') || ''
    
    // Return the expected cookie name format for the HTTP interceptor to use
    return clusterId ? `kube-bind-${clusterId}` : 'kube-bind'
  }

  isCliFlow(): boolean {
      const urlParams = new URLSearchParams(window.location.search)
      return urlParams.has('redirect_url')
  }

  redirectToCliCallback(bindingResponseData: any): void {
    const urlParams = new URLSearchParams(window.location.search)
    const redirectUrl = urlParams.get('redirect_url')
    const sessionId = urlParams.get('session_id')
    if (redirectUrl) {
      const callbackUrl = new URL(redirectUrl)
      if (sessionId) {
        callbackUrl.searchParams.append('session_id', sessionId)
      }
      
      // Add binding response data as base64 encoded query parameter
      const base64Response = btoa(JSON.stringify(bindingResponseData))
      callbackUrl.searchParams.append('binding_response', base64Response)
      
      window.location.href = callbackUrl.toString()
    }
  }
}

export const authService = new AuthService()

import { httpClient, StructuredError } from './http'
import { ErrorCodes } from '../types/binding'

export interface AuthStatus {
  isAuthenticated: boolean
  sessionId?: string
  clusterId?: string
}

export interface AuthCheckResult {
  isAuthenticated: boolean
  error?: string
}

class AuthService {
  async isAuthenticated(): Promise<boolean> {
    const result = await this.checkAuthentication()
    return result.isAuthenticated
  }

  async checkAuthentication(): Promise<AuthCheckResult> {
    try {
      // Since cookies are HTTP-only, we need to check authentication by making an API call
      const urlParams = new URLSearchParams(window.location.search)
      const clusterId = urlParams.get('cluster_id') || ''
      const consumerId = urlParams.get('consumer_id') || ''
      // Make a simple API call to check if we're authenticated
      const authCheckUrl = clusterId ? `/ping?cluster_id=${clusterId}` : '/ping'
      const response = await httpClient.get(authCheckUrl)
      const isAuth = response.status === 200
      console.log('Auth check:', { clusterId, status: response.status, isAuth })
      return { isAuthenticated: isAuth }
    } catch (error) {
      // Handle structured errors
      if (error instanceof StructuredError) {
        const kubeError = error.kubeBindError

        // Return structured error message for auth/authorization failures
        if (kubeError.code === ErrorCodes.AUTHENTICATION_FAILED || kubeError.code === ErrorCodes.AUTHORIZATION_FAILED) {
          return {
            isAuthenticated: false,
            error: kubeError.message
          }
        }
      }

      // Handle cases where we get 401/403 but don't have the expected structured error
      if ((error as any)?.response?.status === 401 || (error as any)?.response?.status === 403) {
        console.log('Auth check failed with authentication error but no structured response')
        return {
          isAuthenticated: false,
          error: 'Authentication required'
        }
      }

      console.error('Auth check error:', error)
      return {
        isAuthenticated: false,
        error: 'Authentication check failed'
      }
    }
  }

  async initiateAuth(
    sessionId: string,
    clusterId: string,
    consumerId?: string,
  ): Promise<void> {
    const authUrl = `/api/authorize`

    const redirect_url = window.location.origin + window.location.pathname

    // Store query parameters in sessionStorage to preserve them through OAuth flow
    const currentParams = new URLSearchParams(window.location.search)
    const paramsToPreserve: Record<string, string> = {}

    // Store all query params that we need to preserve
    if (currentParams.has('consumer_id')) {
      paramsToPreserve.consumer_id = currentParams.get('consumer_id')!
    }
    if (currentParams.has('session_id')) {
      paramsToPreserve.session_id = currentParams.get('session_id')!
    }
    if (currentParams.has('redirect_url')) {
      paramsToPreserve.redirect_url = currentParams.get('redirect_url')!
    }
    if (currentParams.has('cluster_id')) {
      paramsToPreserve.cluster_id = currentParams.get('cluster_id')!
    }

    sessionStorage.setItem('kube-bind-preserved-params', JSON.stringify(paramsToPreserve))

    const params = new URLSearchParams({
      session_id: sessionId,
      redirect_url: redirect_url,
      cluster_id: clusterId,
      client_type: 'ui' // Use UI type to get cookies
    })

    if (consumerId) {
      params.set('consumer_id', consumerId)
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
      const response = await httpClient.post(logoutUrl)

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
      // Check URL params first
      if (urlParams.has('redirect_url')) {
        return true
      }
      
      // Check sessionStorage in case params were lost during OAuth flow
      const preservedParams = sessionStorage.getItem('kube-bind-preserved-params')
      if (preservedParams) {
        try {
          const params = JSON.parse(preservedParams)
          return !!params.redirect_url
        } catch (e) {
          return false
        }
      }
      
      return false
  }

  redirectToCliCallback(bindingResponseData: any): void {
    // Try to get parameters from URL first, then fall back to sessionStorage
    const urlParams = new URLSearchParams(window.location.search)
    let redirectUrl = urlParams.get('redirect_url')
    let sessionId = urlParams.get('session_id')
    let consumerId = urlParams.get('consumer_id')
    
    // If not in URL, check sessionStorage (in case they were lost during OAuth flow)
    if (!redirectUrl) {
      const preservedParams = sessionStorage.getItem('kube-bind-preserved-params')
      if (preservedParams) {
        try {
          const params = JSON.parse(preservedParams)
          redirectUrl = params.redirect_url || null
          sessionId = sessionId || params.session_id || null
          consumerId = consumerId || params.consumer_id || null
          console.log('Retrieved CLI params from sessionStorage:', { redirectUrl, sessionId, consumerId })
        } catch (e) {
          console.error('Failed to parse preserved params:', e)
        }
      }
    }
    
    if (redirectUrl) {
      // Construct the callback URL entirely on the client side
      const callbackUrl = new URL(redirectUrl)
      
      if (sessionId) {
        callbackUrl.searchParams.append('session_id', sessionId)
      }
      if (consumerId) {
        callbackUrl.searchParams.append('consumer_id', consumerId)
      }

      // Add binding response data as base64 encoded query parameter
      const base64Response = btoa(JSON.stringify(bindingResponseData))
      callbackUrl.searchParams.append('binding_response', base64Response)

      console.log('Redirecting to CLI callback:', callbackUrl.toString())
      window.location.href = callbackUrl.toString()
    } else {
      console.error('No redirect URL found for CLI callback')
    }
  }

  restorePreservedParams(): void {
    // Check if we have preserved params from before OAuth redirect
    const preservedParamsJson = sessionStorage.getItem('kube-bind-preserved-params')

    if (!preservedParamsJson) {
      return
    }

    try {
      const preservedParams = JSON.parse(preservedParamsJson)
      const currentParams = new URLSearchParams(window.location.search)

      // Only restore if the params are missing in current URL
      let needsUpdate = false

      for (const [key, value] of Object.entries(preservedParams)) {
        if (!currentParams.has(key)) {
          currentParams.set(key, value as string)
          needsUpdate = true
        }
      }

      if (needsUpdate) {
        // Clear the stored params
        sessionStorage.removeItem('kube-bind-preserved-params')

        // Update URL with preserved params and reload to ensure Vue Router picks up the changes
        const newUrl = `${window.location.pathname}?${currentParams.toString()}`
        window.location.replace(newUrl)
      } else {
        // Params are already in URL, just clean up storage
        sessionStorage.removeItem('kube-bind-preserved-params')
      }
    } catch (error) {
      console.error('Failed to restore preserved params:', error)
      sessionStorage.removeItem('kube-bind-preserved-params')
    }
  }
}

export const authService = new AuthService()

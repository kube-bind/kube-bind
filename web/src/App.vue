<template>
  <div id="app">
    <header class="header">
      <div class="header-content">
        <div class="brand">
          <div class="logo">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
              <path d="M2 17L12 22L22 17" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
              <path d="M2 12L12 17L22 12" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
            </svg>
          </div>
          <h1>Kube Bind</h1>
        </div>
        <div v-if="authStatus.isAuthenticated" class="user-section">
          <div class="user-info">
            <div class="status-indicator"></div>
            <span class="welcome-text">Connected</span>
          </div>
          <button @click="logout" class="logout-btn">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M9 21H5C4.46957 21 3.96086 20.7893 3.58579 20.4142C3.21071 20.0391 3 19.5304 3 19V5C3 4.46957 3.21071 3.96086 3.58579 3.58579C3.96086 3.21071 4.46957 3 5 3H9" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M16 17L21 12L16 7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M21 12H9" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            Sign out
          </button>
        </div>
      </div>
    </header>
    
    <main class="main">
      <div v-if="!authStatus.isAuthenticated" class="auth-placeholder">
        <h2>Authentication Required</h2>
        <p>Please authenticate to access resources.</p>
        <button @click="authenticate" class="auth-btn">Authenticate</button>
      </div>
      
      <router-view v-else :auth-status="authStatus"></router-view>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { authService } from './services/auth'

const route = useRoute()

const authStatus = ref({
  isAuthenticated: false,
  loading: true,
  error: null as string | null
})

const checkAuthStatus = async () => {
  try {
    const authenticated = await authService.isAuthenticated()
    authStatus.value.isAuthenticated = authenticated
  } catch (error) {
    console.error('Auth check failed:', error)
    authStatus.value.error = 'Authentication check failed'
  } finally {
    authStatus.value.loading = false
  }
}

const authenticate = async () => {
  try {
    const cluster = route.query.cluster_id as string || ''
    const sessionId = route.query.session_id as string || generateSessionId()
    const clientSideRedirectUrl = route.query.redirect_url as string || ''

    await authService.initiateAuth(sessionId, cluster, clientSideRedirectUrl)
  } catch (error) {
    console.error('Authentication failed:', error)
    authStatus.value.error = 'Authentication failed'
  }
}

const logout = async () => {
  try {
    await authService.logout()
    authStatus.value.isAuthenticated = false
  } catch (error) {
    console.error('Logout failed:', error)
  }
}

const generateSessionId = (): string => {
  return Math.random().toString(36).substring(2) + Date.now().toString(36)
}

onMounted(() => {
  checkAuthStatus()
  
  // Listen for authentication expiration events from HTTP interceptor
  window.addEventListener('auth-expired', () => {
    console.log('Received auth-expired event, updating authentication status')
    authStatus.value.isAuthenticated = false
    authStatus.value.error = 'Session expired. Please re-authenticate.'
  })
})
</script>

<style scoped>
#app {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
}

.header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.logo {
  color: rgba(255, 255, 255, 0.9);
  display: flex;
  align-items: center;
  justify-content: center;
}

.header h1 {
  margin: 0;
  color: white;
  font-size: 1.5rem;
  font-weight: 600;
  letter-spacing: -0.025em;
}

.user-section {
  display: flex;
  align-items: center;
  gap: 1.5rem;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: rgba(255, 255, 255, 0.9);
  font-size: 0.875rem;
  font-weight: 500;
}

.status-indicator {
  width: 8px;
  height: 8px;
  background: #10b981;
  border-radius: 50%;
  box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.3);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.welcome-text {
  color: rgba(255, 255, 255, 0.8);
}

.logout-btn {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s ease;
  backdrop-filter: blur(10px);
}

.logout-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: white;
  border-color: rgba(255, 255, 255, 0.3);
  transform: translateY(-1px);
}

.main {
  min-height: calc(100vh - 80px);
  background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
  padding: 0;
}

.auth-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 4rem 2rem;
  min-height: calc(100vh - 200px);
}

.auth-placeholder h2 {
  color: #1f2937;
  margin-bottom: 1rem;
  font-size: 2rem;
  font-weight: 600;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.auth-placeholder p {
  color: #6b7280;
  margin-bottom: 2rem;
  font-size: 1.125rem;
  max-width: 500px;
  line-height: 1.6;
}

.auth-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: none;
  border-radius: 12px;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 600;
  transition: all 0.2s ease;
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
}

.auth-btn:hover {
  background: linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%);
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(102, 126, 234, 0.4);
}
</style>
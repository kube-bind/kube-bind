<template>
  <div class="container">
    <div class="exports-header">
      <div class="breadcrumb" v-if="clusterId">
        <router-link to="/clusters" class="breadcrumb-link">Clusters</router-link>
        <span class="breadcrumb-separator">→</span>
        <router-link :to="`/clusters/${clusterId}`" class="breadcrumb-link">{{ clusterId }}</router-link>
        <span class="breadcrumb-separator">→</span>
        <span class="breadcrumb-current">Exports</span>
      </div>
      <h1>Service Exports</h1>
      <p v-if="clusterId">Binding provider information for cluster: <span class="cluster-name">{{ clusterId }}</span></p>
      <p v-else>View binding provider information and authentication methods</p>
    </div>

    <div v-if="loading" class="loading">
      <p>Loading exports...</p>
    </div>

    <div v-else-if="error" class="error">
      <h3>Error loading exports</h3>
      <p>{{ error }}</p>
      <button @click="loadExports" class="btn btn-primary">Retry</button>
    </div>

    <div v-else-if="exports" class="exports-content">
      <div class="provider-card">
        <div class="provider-header">
          <div class="provider-icon">🏢</div>
          <div class="provider-info">
            <h2 class="provider-name">{{ exports.ProviderPrettyName || 'Service Provider' }}</h2>
            <p class="provider-version">Version: {{ exports.Version || 'Unknown' }}</p>
          </div>
        </div>

        <div class="provider-details">
          <div class="detail-section">
            <h3>Provider Information</h3>
            <div class="provider-meta">
              <div class="meta-item">
                <span class="meta-label">API Version:</span>
                <span class="meta-value">{{ exports.APIVersion }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Kind:</span>
                <span class="meta-value">{{ exports.Kind }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Version:</span>
                <span class="meta-value">{{ exports.Version }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">Provider Name:</span>
                <span class="meta-value">{{ exports.ProviderPrettyName }}</span>
              </div>
            </div>
          </div>

          <div class="detail-section" v-if="exports.AuthenticationMethods && exports.AuthenticationMethods.length > 0">
            <h3>Authentication Methods</h3>
            <div class="auth-methods">
              <div
                v-for="(method, index) in exports.AuthenticationMethods"
                :key="index"
                class="auth-method-card"
              >
                <div class="auth-method-header">
                  <div class="auth-icon">🔐</div>
                  <h4>{{ method.Method }}</h4>
                </div>
                
                <div v-if="method.OAuth2CodeGrant" class="auth-details">
                  <div class="auth-meta">
                    <div class="meta-item">
                      <span class="meta-label">Authentication URL:</span>
                      <span class="meta-value">{{ method.OAuth2CodeGrant.AuthenticatedURL }}</span>
                    </div>
                  </div>
                  
                  <div class="auth-actions">
                    <button @click="startAuthentication(method.OAuth2CodeGrant.AuthenticatedURL)" class="btn btn-primary">
                      Start Authentication
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="provider-actions">
            <button @click="loadExports" class="btn btn-secondary">
              Refresh
            </button>
            <router-link v-if="clusterId" :to="`/clusters/${clusterId}/resources`" class="btn btn-primary">
              View Resources
            </router-link>
            <router-link v-else to="/resources" class="btn btn-primary">
              View Resources
            </router-link>
          </div>
        </div>
      </div>

      <div class="export-info">
        <div class="info-card">
          <h3>About Service Exports</h3>
          <p>
            Service exports define the API contracts and authentication methods available from this provider.
            Use the authentication methods above to gain access to the resources provided by this service.
          </p>
        </div>
      </div>
    </div>

    <div v-else class="empty-state">
      <div class="empty-icon">📤</div>
      <h3>No Exports Available</h3>
      <p>There are no service exports available at this time.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { authService } from '../services/auth'

interface Props {
  cluster?: string
}

interface OAuth2CodeGrant {
  AuthenticatedURL: string
}

interface AuthenticationMethod {
  Method: string
  OAuth2CodeGrant?: OAuth2CodeGrant
}

interface ExportData {
  APIVersion: string
  Kind: string
  Version: string
  ProviderPrettyName: string
  AuthenticationMethods: AuthenticationMethod[]
}

const props = defineProps<Props>()
const router = useRouter()
const route = useRoute()

const exports = ref<ExportData | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const clusterId = computed(() => {
  return props.cluster || (route.params.cluster as string) || ''
})

const loadExports = async () => {
  loading.value = true
  error.value = null
  
  try {
    const data = await authService.getExports(clusterId.value)
    exports.value = data
  } catch (err) {
    console.error('Failed to load exports:', err)
    error.value = err instanceof Error ? err.message : 'Failed to load exports'
  } finally {
    loading.value = false
  }
}

const startAuthentication = (authUrl: string) => {
  if (authUrl) {
    window.location.href = authUrl
  }
}

onMounted(() => {
  loadExports()
})
</script>

<style scoped>
.exports-header {
  text-align: center;
  margin-bottom: 3rem;
}

.exports-header h1 {
  font-size: 2.5rem;
  font-weight: 700;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.exports-header p {
  font-size: 1.1rem;
  color: #718096;
}

.breadcrumb {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}

.breadcrumb-link {
  color: #0366d6;
  text-decoration: none;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.breadcrumb-link:hover {
  background-color: #f6f8fa;
}

.breadcrumb-separator {
  margin: 0 0.5rem;
  color: #6b7280;
}

.breadcrumb-current {
  color: #374151;
  font-weight: 500;
}

.cluster-name {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  color: #0366d6;
  font-weight: 600;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: #6b7280;
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
}

.empty-state h3 {
  font-size: 1.5rem;
  margin-bottom: 1rem;
  color: #374151;
}

.exports-content {
  max-width: 800px;
  margin: 0 auto;
}

.provider-card {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
  margin-bottom: 2rem;
}

.provider-header {
  display: flex;
  align-items: center;
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #e1e4e8;
}

.provider-icon {
  font-size: 3rem;
  margin-right: 1.5rem;
}

.provider-name {
  font-size: 1.8rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.provider-version {
  color: #718096;
  font-size: 1rem;
}

.detail-section {
  margin-bottom: 2rem;
}

.detail-section h3 {
  font-size: 1.3rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 1rem;
}

.provider-meta {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 0;
  border-bottom: 1px solid #f1f5f9;
}

.meta-item:last-child {
  border-bottom: none;
}

.meta-label {
  font-weight: 500;
  color: #4a5568;
}

.meta-value {
  color: #2d3748;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.3rem 0.6rem;
  border-radius: 4px;
  font-size: 0.9rem;
}

.auth-methods {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.auth-method-card {
  background: #f8f9fa;
  border-radius: 8px;
  padding: 1.5rem;
  border: 1px solid #e9ecef;
}

.auth-method-header {
  display: flex;
  align-items: center;
  margin-bottom: 1rem;
}

.auth-icon {
  font-size: 1.5rem;
  margin-right: 0.75rem;
}

.auth-method-header h4 {
  font-size: 1.1rem;
  font-weight: 600;
  color: #2c3e50;
}

.auth-details {
  margin-top: 1rem;
}

.auth-meta {
  margin-bottom: 1rem;
}

.auth-actions {
  margin-top: 1rem;
}

.provider-actions {
  display: flex;
  gap: 1rem;
  padding-top: 1rem;
  border-top: 1px solid #e1e4e8;
}

.info-card {
  background: #f8f9fa;
  border-radius: 12px;
  padding: 2rem;
  border: 1px solid #e9ecef;
}

.info-card h3 {
  font-size: 1.2rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 1rem;
}

.info-card p {
  color: #6c757d;
  line-height: 1.6;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background-color: #5a6268;
}

@media (max-width: 768px) {
  .provider-actions {
    flex-direction: column;
  }
  
  .provider-header {
    flex-direction: column;
    text-align: center;
  }
  
  .provider-icon {
    margin-right: 0;
    margin-bottom: 1rem;
  }
}
</style>
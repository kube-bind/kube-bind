<template>
  <div class="container">
    <div class="resources-header">
      <h1>Available Resources</h1>
      <p v-if="clusterId">Resources for cluster: <span class="cluster-name">{{ clusterId }}</span></p>
      <p v-else>Select resources to bind to your cluster</p>
    </div>

    <div v-if="loading" class="loading">
      <p>Loading resources...</p>
    </div>

    <div v-else-if="error" class="error">
      <h3>Error loading resources</h3>
      <p>{{ error }}</p>
      <button @click="loadResources" class="btn btn-primary">Retry</button>
    </div>

    <div v-else-if="resources.length === 0" class="empty-state">
      <div class="empty-icon">📦</div>
      <h3>No Resources Available</h3>
      <p>There are no resources available for binding at this time.</p>
    </div>

    <div v-else class="resources-grid">
      <div
        v-for="resource in resources"
        :key="`${resource.group}.${resource.resource}`"
        class="resource-card"
      >
      
        <div class="resource-header">
          <div class="resource-icon">🔧</div>
          <div class="resource-info">
            <h3 class="resource-name">{{ resource.name }}</h3>
            <p class="resource-kind">{{ resource.kind }}</p>
          </div>
        </div>

        <div class="resource-details">
          <div class="resource-meta">
            <div class="meta-item">
              <span class="meta-label">Group:</span>
              <span class="meta-value">{{ resource.group || 'core' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Version:</span>
              <span class="meta-value">{{ resource.version }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Scope:</span>
              <span class="meta-value">{{ resource.scope }}</span>
            </div>
          </div>
        </div>

        <div class="resource-actions">
          <button
            @click="bindResource(resource)"
            class="btn btn-primary btn-full"
            :disabled="binding === resource.resource"
          >
            <span v-if="binding === resource.resource">Binding...</span>
            <span v-else>Bind Resource</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { authService } from '../services/auth'

interface Props {
  cluster?: string
}

interface UISchema {
  name: string
  version: string
  group: string
  kind: string
  scope: string
  resource: string
  sessionID: string
}

const props = defineProps<Props>()
const route = useRoute()

const resources = ref<UISchema[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const binding = ref<string | null>(null)

const clusterId = computed(() => {
  return props.cluster || (route.params.cluster as string) || ''
})

const sessionId = computed(() => {
  return (route.query.s as string) || ''
})

const loadResources = async () => {
  console.log('🔄 Loading resources...')
  console.log('📋 Current state:', { 
    clusterId: clusterId.value, 
    sessionId: sessionId.value,
    route: route.path,
    query: route.query,
    params: route.params
  })
  
  loading.value = true
  error.value = null
  
  try {
    // If we have a session ID from the URL, use it directly
    const currentSessionId = sessionId.value
    if (currentSessionId) {
      console.log('🔑 Using session ID from URL:', currentSessionId)
      const data = await authService.getResourcesWithSession(clusterId.value, currentSessionId)
      console.log('📦 Raw API response:', data)
      
      // Handle response format - check for both 'resources' (lowercase) and 'Resources' (uppercase)
      if (data && typeof data === 'object' && 'resources' in data && Array.isArray(data.resources)) {
        console.log('✅ Found resources in response.resources:', data.resources.length, 'items')
        resources.value = data.resources
      } else if (data && typeof data === 'object' && 'Resources' in data && Array.isArray(data.Resources)) {
        console.log('✅ Found resources in response.Resources:', data.Resources.length, 'items')
        resources.value = data.Resources
      } else if (Array.isArray(data)) {
        console.log('✅ Found resources as direct array:', data.length, 'items')
        resources.value = data
      } else {
        console.log('❌ Unexpected response format:', data)
        resources.value = []
      }
    } else {
      console.log('🔍 No session ID, checking authentication...')
      // Fallback to checking authentication status
      const isAuth = await authService.checkAuthentication()
      if (!isAuth) {
        console.log('❌ Not authenticated')
        error.value = 'No session found. Please authenticate first.'
        return
      }

      console.log('✅ Authenticated, fetching resources...')
      const data = await authService.getResources(clusterId.value)
      console.log('📦 Raw API response (fallback):', data)
      
      // Handle response format - check for both 'resources' (lowercase) and 'Resources' (uppercase)
      if (data && typeof data === 'object' && 'resources' in data && Array.isArray(data.resources)) {
        resources.value = data.resources
      } else if (data && typeof data === 'object' && 'Resources' in data && Array.isArray(data.Resources)) {
        resources.value = data.Resources
      } else if (Array.isArray(data)) {
        resources.value = data
      } else {
        resources.value = []
      }
    }
    
    console.log('📊 Final resources count:', resources.value.length)
  } catch (err) {
    console.error('❌ Failed to load resources:', err)
    error.value = err instanceof Error ? err.message : 'Failed to load resources'
  } finally {
    loading.value = false
    console.log('🏁 Loading complete')
  }
}

const bindResource = async (resource: UISchema) => {
  binding.value = resource.resource
  console.log('🔗 Starting bind process for resource:', resource)
  
  try {
    const currentSessionId = sessionId.value
    let response
    
    if (currentSessionId) {
      console.log('🔑 Using session ID for binding')
      response = await authService.bindResourceWithSession(
        resource.group, 
        resource.resource, 
        resource.version, 
        clusterId.value, 
        currentSessionId,
        resource.scope,
        resource.kind,
        resource.name
      )
    } else {
      console.log('🍪 Using cookie-based authentication for binding')
      response = await authService.bindResource(resource.group, resource.resource, resource.version, clusterId.value)
    }
    
    console.log('✅ Bind operation completed successfully')
    console.log('📦 Bind response:', response)
    
    // Handle the response - could be a redirect URL or success message
    if (response && typeof response === 'string' && response.startsWith('http')) {
      console.log('🔄 Redirecting to:', response)
      window.location.href = response
    } else if (response && response.redirectURL) {
      console.log('🔄 Redirecting to:', response.redirectURL)
      window.location.href = response.redirectURL
    } else {
      console.log('✅ Bind completed, no redirect needed')
      // Show success message or refresh resources
      // Could implement a success toast notification here
    }
    
  } catch (err) {
    console.error('❌ Failed to bind resource:', err)
    error.value = err instanceof Error ? err.message : 'Failed to bind resource'
  } finally {
    binding.value = null
  }
}

// Watch for route changes to reload resources automatically
watch([clusterId, sessionId], () => {
  loadResources()
}, { immediate: true })

onMounted(() => {
  loadResources()
})
</script>

<style scoped>
.resources-header {
  text-align: center;
  margin-bottom: 3rem;
}

.cluster-name {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  color: #0366d6;
  font-weight: 600;
}

.resources-header h1 {
  font-size: 2.5rem;
  font-weight: 700;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.resources-header p {
  font-size: 1.1rem;
  color: #718096;
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

.resources-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 1.5rem;
}

.resource-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
  transition: transform 0.2s, box-shadow 0.2s;
}

.resource-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 15px rgba(0, 0, 0, 0.1);
}

.resource-header {
  display: flex;
  align-items: center;
  margin-bottom: 1rem;
}

.resource-icon {
  font-size: 2rem;
  margin-right: 1rem;
}

.resource-info h3 {
  font-size: 1.2rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.25rem;
}

.resource-kind {
  color: #718096;
  font-size: 0.9rem;
}

.resource-details {
  margin-bottom: 1.5rem;
}

.resource-meta {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 0;
  border-bottom: 1px solid #f1f5f9;
}

.meta-item:last-child {
  border-bottom: none;
}

.meta-label {
  font-weight: 500;
  color: #4a5568;
  font-size: 0.85rem;
}

.meta-value {
  color: #2d3748;
  font-size: 0.85rem;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
}

.resource-actions {
  margin-top: 1rem;
}

.btn-full {
  width: 100%;
  padding: 0.75rem;
  font-size: 0.9rem;
}

@media (max-width: 768px) {
  .resources-grid {
    grid-template-columns: 1fr;
  }
  
  .resource-card {
    margin: 0 1rem;
  }
}
</style>
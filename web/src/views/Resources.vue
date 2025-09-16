<template>
  <div class="container">
    <div class="resources-header">
      <div class="breadcrumb" v-if="clusterId">
        <router-link to="/clusters" class="breadcrumb-link">Clusters</router-link>
        <span class="breadcrumb-separator">→</span>
        <router-link :to="`/clusters/${clusterId}`" class="breadcrumb-link">{{ clusterId }}</router-link>
        <span class="breadcrumb-separator">→</span>
        <span class="breadcrumb-current">Resources</span>
      </div>
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
        :key="`${resource.Group}.${resource.Resource}`"
        class="resource-card"
      >
        <div class="resource-header">
          <div class="resource-icon">🔧</div>
          <div class="resource-info">
            <h3 class="resource-name">{{ resource.Name }}</h3>
            <p class="resource-kind">{{ resource.Kind }}</p>
          </div>
        </div>

        <div class="resource-details">
          <div class="resource-meta">
            <div class="meta-item">
              <span class="meta-label">Group:</span>
              <span class="meta-value">{{ resource.Group || 'core' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Version:</span>
              <span class="meta-value">{{ resource.Version }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Scope:</span>
              <span class="meta-value">{{ resource.Scope }}</span>
            </div>
          </div>
        </div>

        <div class="resource-actions">
          <button
            @click="bindResource(resource)"
            class="btn btn-primary btn-full"
            :disabled="binding === resource.Resource"
          >
            <span v-if="binding === resource.Resource">Binding...</span>
            <span v-else>Bind Resource</span>
          </button>
        </div>
      </div>
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

interface UISchema {
  Name: string
  Version: string
  Group: string
  Kind: string
  Scope: string
  Resource: string
  SessionID: string
}

const props = defineProps<Props>()
const router = useRouter()
const route = useRoute()

const resources = ref<UISchema[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const binding = ref<string | null>(null)

const clusterId = computed(() => {
  return props.cluster || (route.params.cluster as string) || ''
})

const loadResources = async () => {
  loading.value = true
  error.value = null
  
  try {
    const isAuth = await authService.checkAuthentication()
    if (!isAuth) {
      router.push('/login')
      return
    }

    const data = await authService.getResources(clusterId.value)
    resources.value = data
  } catch (err) {
    console.error('Failed to load resources:', err)
    error.value = err instanceof Error ? err.message : 'Failed to load resources'
  } finally {
    loading.value = false
  }
}

const bindResource = async (resource: UISchema) => {
  binding.value = resource.Resource
  
  try {
    await authService.bindResource(resource.Group, resource.Resource, resource.Version, clusterId.value)
  } catch (err) {
    console.error('Failed to bind resource:', err)
    error.value = err instanceof Error ? err.message : 'Failed to bind resource'
  } finally {
    binding.value = null
  }
}

onMounted(() => {
  loadResources()
})
</script>

<style scoped>
.resources-header {
  text-align: center;
  margin-bottom: 3rem;
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
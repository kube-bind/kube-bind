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
              <span class="meta-value">{{ resource.apiVersion }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Scope:</span>
              <span class="meta-value">{{ resource.scope }}</span>
            </div>
          </div>
        </div>

        <div class="resource-actions">
          <div class="name-input-section">
            <label class="name-label" :for="`name-${resource.resource}`">Custom Name (optional):</label>
            <input
              :id="`name-${resource.resource}`"
              v-model="resourceNames[resource.resource]"
              :placeholder="`Default: ${resource.name}`"
              :class="[
                'name-input',
                { 'name-input-error': getNameValidationMessage(resourceNames[resource.resource] || '') }
              ]"
              type="text"
              maxlength="253"
            >
            <small 
              v-if="getNameValidationMessage(resourceNames[resource.resource] || '')"
              class="name-error"
            >
              {{ getNameValidationMessage(resourceNames[resource.resource] || '') }}
            </small>
            <small v-else class="name-hint">Used to identify this binding request</small>
          </div>
          
          <div class="action-buttons">
            <button
              @click="openPermissionModal(resource)"
              class="btn btn-secondary btn-full"
              style="margin-bottom: 0.5rem;"
            >
              Configure Permissions
              <span v-if="getConfiguredClaimsCount(resource.resource) > 0" class="claims-count">
                ({{ getConfiguredClaimsCount(resource.resource) }})
              </span>
            </button>
            
            <button
              @click="bindResource(resource)"
              class="btn btn-primary btn-full"
              :disabled="binding === resource.resource || !isValidName(resourceNames[resource.resource] || '')"
            >
              <span v-if="binding === resource.resource">Binding...</span>
              <span v-else>Bind Resource</span>
            </button>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Permission Claims Modal -->
    <div v-if="showModal" class="modal-overlay" @click="closeModal">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>Configure Permission Claims</h3>
          <button @click="closeModal" class="modal-close">&times;</button>
        </div>
        
        <div class="modal-body">
          <p class="modal-description">
            Select additional resources that <strong>{{ currentResource?.name }}</strong> can access:
          </p>
          
          <div v-if="claimableResources.length > 0" class="claims-list">
            <div
              v-for="claimable in claimableResources"
              :key="`${claimable.groupVersionResource?.group || 'core'}.${claimable.groupVersionResource?.resource}`"
              class="claim-item"
            >
              <div class="claim-header">
                <label class="claim-checkbox">
                  <input
                    type="checkbox"
                    :value="`${claimable.groupVersionResource?.group || 'core'}.${claimable.groupVersionResource?.resource}`"
                    v-model="modalSelectedClaims"
                    @change="onClaimToggle($event, claimable)"
                  >
                  <span class="claim-label">{{ claimable.names?.kind }}</span>
                  <span class="claim-details">({{ claimable.groupVersionResource?.group || 'core' }}/{{ claimable.groupVersionResource?.resource }})</span>
                </label>
              </div>
              
              <div 
                v-if="isClaimSelected(claimable)" 
                class="claim-config"
              >
                <div class="selector-options">
                  <label class="radio-option">
                    <input
                      type="radio"
                      :name="`selector-${claimable.groupVersionResource?.resource}`"
                      value="all"
                      v-model="claimSelectors[getClaimKey(claimable)]"
                      @change="updateClaimSelector(claimable, 'all')"
                    >
                    <span>All resources</span>
                  </label>
                  
                  <label class="radio-option">
                    <input
                      type="radio"
                      :name="`selector-${claimable.groupVersionResource?.resource}`"
                      value="labels"
                      v-model="claimSelectors[getClaimKey(claimable)]"
                      @change="updateClaimSelector(claimable, 'labels')"
                    >
                    <span>Select by labels</span>
                  </label>
                </div>
                
                <div 
                  v-if="claimSelectors[getClaimKey(claimable)] === 'labels'"
                  class="label-selector-config"
                >
                  <div class="label-pairs">
                    <div 
                      v-for="(labelPair, index) in claimLabels[getClaimKey(claimable)] || []"
                      :key="index"
                      class="label-pair"
                    >
                      <input
                        v-model="labelPair.key"
                        placeholder="Label key"
                        class="label-input"
                      >
                      <input
                        v-model="labelPair.value"
                        placeholder="Label value"
                        class="label-input"
                      >
                      <button 
                        @click="removeLabelPair(claimable, index)"
                        class="btn-remove"
                      >
                        ×
                      </button>
                    </div>
                  </div>
                  
                  <button 
                    @click="addLabelPair(claimable)"
                    class="btn btn-sm btn-secondary"
                  >
                    Add Label
                  </button>
                </div>
              </div>
            </div>
          </div>
          
          <div v-else class="claims-loading">
            <p>Loading available permissions...</p>
          </div>
        </div>
        
        <div class="modal-footer">
          <button @click="closeModal" class="btn btn-secondary">Cancel</button>
          <button @click="savePermissionClaims" class="btn btn-primary">Save</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { authService, type ClaimableResource, type PermissionClaim } from '../services/auth'

interface Props {
  cluster?: string
}

interface UISchema {
  name: string
  apiVersion: string
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
const claimableResources = ref<ClaimableResource[]>([])

// Modal state
const showModal = ref(false)
const currentResource = ref<UISchema | null>(null)
const modalSelectedClaims = ref<string[]>([])

// Permission claims configuration
const configuredClaims = ref<Record<string, PermissionClaim[]>>({})
const claimSelectors = ref<Record<string, 'all' | 'labels'>>({})
const claimLabels = ref<Record<string, Array<{ key: string, value: string }>>>({})

// Custom names for binding requests
const resourceNames = ref<Record<string, string>>({})

interface LabelPair {
  key: string
  value: string
}

const clusterId = computed(() => {
  return props.cluster || (route.params.cluster as string) || ''
})

const sessionId = computed(() => {
  return (route.query.s as string) || ''
})

// Name validation
const isValidName = (name: string): boolean => {
  if (!name.trim()) return true // Empty is allowed (optional)
  
  // Kubernetes name validation: lowercase alphanumeric, dashes, dots, max 253 chars
  const nameRegex = /^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$/
  return nameRegex.test(name.trim()) && name.trim().length <= 253
}

const getNameValidationMessage = (name: string): string => {
  if (!name.trim()) return ''
  if (!isValidName(name)) {
    return 'Name must contain only lowercase letters, numbers, dashes, and dots'
  }
  return ''
}

const loadClaimableResources = async () => {
  try {
    claimableResources.value = await authService.getClaimableResources()
    console.log('✅ Loaded claimable resources:', claimableResources.value.length, 'items')
  } catch (err) {
    console.error('❌ Failed to load claimable resources:', err)
    // Don't block the main loading if claimable resources fail
  }
}

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
    // Load claimable resources in parallel
    loadClaimableResources()
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
        console.log('❌ Not authenticated')
        error.value = 'No session found. Please authenticate first.'
        return
    }
    
    console.log('📊 Final resources count:', resources.value.length)
    
    // Initialize configuredClaims and resourceNames for each resource
    resources.value.forEach(resource => {
      if (!configuredClaims.value[resource.resource]) {
        configuredClaims.value[resource.resource] = []
      }
      if (!resourceNames.value[resource.resource]) {
        resourceNames.value[resource.resource] = ''
      }
    })
  } catch (err) {
    console.error('❌ Failed to load resources:', err)
    error.value = err instanceof Error ? err.message : 'Failed to load resources'
  } finally {
    loading.value = false
    console.log('🏁 Loading complete')
  }
}

// Modal methods
const openPermissionModal = (resource: UISchema) => {
  currentResource.value = resource
  showModal.value = true
  
  // Initialize modal state with current configured claims
  const currentClaims = configuredClaims.value[resource.resource] || []
  modalSelectedClaims.value = currentClaims.map(claim => `${claim.group || 'core'}.${claim.resource}`)
  
  // Initialize selectors and labels
  currentClaims.forEach(claim => {
    const key = `${claim.group || 'core'}.${claim.resource}`
    if (claim.selector?.all) {
      claimSelectors.value[key] = 'all'
    } else if (claim.selector?.labelSelector) {
      claimSelectors.value[key] = 'labels'
      claimLabels.value[key] = Object.entries(claim.selector.labelSelector.matchLabels || {}).map(([k, v]) => ({ key: k, value: v }))
    } else {
      claimSelectors.value[key] = 'all'
    }
  })
}

const closeModal = () => {
  showModal.value = false
  currentResource.value = null
  modalSelectedClaims.value = []
  // Clear temporary state
  Object.keys(claimSelectors.value).forEach(key => {
    delete claimSelectors.value[key]
    delete claimLabels.value[key]
  })
}

const getClaimKey = (claimable: ClaimableResource) => {
  return `${claimable.groupVersionResource?.group || 'core'}.${claimable.groupVersionResource?.resource}`
}

const isClaimSelected = (claimable: ClaimableResource) => {
  const key = getClaimKey(claimable)
  return modalSelectedClaims.value.includes(key)
}

const onClaimToggle = (event: Event, claimable: ClaimableResource) => {
  const key = getClaimKey(claimable)
  const isChecked = (event.target as HTMLInputElement).checked
  
  if (isChecked) {
    // Initialize with 'all' selector by default
    claimSelectors.value[key] = 'all'
    claimLabels.value[key] = []
  } else {
    // Clean up when unchecked
    delete claimSelectors.value[key]
    delete claimLabels.value[key]
  }
}

const updateClaimSelector = (claimable: ClaimableResource, selectorType: 'all' | 'labels') => {
  const key = getClaimKey(claimable)
  claimSelectors.value[key] = selectorType
  
  if (selectorType === 'labels' && !claimLabels.value[key]) {
    claimLabels.value[key] = []
  }
}

const addLabelPair = (claimable: ClaimableResource) => {
  const key = getClaimKey(claimable)
  if (!claimLabels.value[key]) {
    claimLabels.value[key] = []
  }
  claimLabels.value[key].push({ key: '', value: '' })
}

const removeLabelPair = (claimable: ClaimableResource, index: number) => {
  const key = getClaimKey(claimable)
  if (claimLabels.value[key]) {
    claimLabels.value[key].splice(index, 1)
  }
}

const savePermissionClaims = () => {
  if (!currentResource.value) return
  
  const resourceKey = currentResource.value.resource
  const claims: PermissionClaim[] = []
  
  modalSelectedClaims.value.forEach(claimKey => {
    const [group, resource] = claimKey.split('.')
    const selectorType = claimSelectors.value[claimKey]
    
    const claim: PermissionClaim = {
      group: group === 'core' ? '' : group,
      resource: resource
    }
    
    if (selectorType === 'all') {
      claim.selector = { all: true }
    } else if (selectorType === 'labels') {
      const labels = claimLabels.value[claimKey] || []
      const matchLabels: Record<string, string> = {}
      
      labels.forEach(({ key, value }) => {
        if (key.trim() && value.trim()) {
          matchLabels[key.trim()] = value.trim()
        }
      })
      
      if (Object.keys(matchLabels).length > 0) {
        claim.selector = {
          labelSelector: { matchLabels }
        }
      } else {
        // If no labels configured, default to all
        claim.selector = { all: true }
      }
    }
    
    claims.push(claim)
  })
  
  configuredClaims.value[resourceKey] = claims
  console.log('💾 Saved permission claims for', resourceKey, ':', claims)
  closeModal()
}

const getConfiguredClaimsCount = (resourceKey: string) => {
  return configuredClaims.value[resourceKey]?.length || 0
}

const bindResource = async (resource: UISchema) => {
  binding.value = resource.resource
  console.log('🔗 Starting bind process for resource:', resource)
  
  try {
    const currentSessionId = sessionId.value
    let response
    
    // Use configured permission claims
    const permissionClaims: PermissionClaim[] = configuredClaims.value[resource.resource] || []
    console.log('🔐 Permission claims for binding:', permissionClaims)
    
    if (currentSessionId) {
      console.log('🔑 Using session ID for binding')
      const customName = resourceNames.value[resource.resource]?.trim()
      console.log('📝 Custom request name:', customName || 'none (using default)')
      
      response = await authService.bindResourceWithSession(
        resource.group, 
        resource.resource, 
        resource.apiVersion, 
        clusterId.value, 
        currentSessionId,
        resource.scope,
        resource.kind,
        resource.name,
        permissionClaims,
        customName
      )
    } else {
      console.log('🍪 Using cookie-based authentication for binding')
      response = await authService.bindResource(resource.group, resource.resource, resource.apiVersion, clusterId.value)
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

.name-input-section {
  margin-bottom: 1rem;
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 6px;
  border: 1px solid #e1e4e8;
}

.name-label {
  display: block;
  font-weight: 500;
  color: #374151;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
}

.name-input {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.name-input:focus {
  outline: none;
  border-color: #0366d6;
  box-shadow: 0 0 0 2px rgba(3, 102, 214, 0.1);
}

.name-input::placeholder {
  color: #9ca3af;
  font-style: italic;
}

.name-hint {
  display: block;
  color: #6b7280;
  font-size: 0.75rem;
  margin-top: 0.25rem;
}

.name-input-error {
  border-color: #dc3545 !important;
  box-shadow: 0 0 0 2px rgba(220, 53, 69, 0.1) !important;
}

.name-error {
  display: block;
  color: #dc3545;
  font-size: 0.75rem;
  margin-top: 0.25rem;
  font-weight: 500;
}

.btn-full {
  width: 100%;
  padding: 0.75rem;
  font-size: 0.9rem;
}

/* Permission claims styling */
.permission-claims-section {
  margin-bottom: 1rem;
  padding: 1rem;
  border: 1px solid #e1e4e8;
  border-radius: 8px;
  background-color: #f8f9fa;
}

.permission-claims-section h4 {
  margin-bottom: 0.5rem;
  color: #2c3e50;
  font-size: 1rem;
}

.claims-description {
  margin-bottom: 1rem;
  color: #6b7280;
  font-size: 0.9rem;
}

.claims-grid {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.claim-option {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  padding: 0.5rem;
}

.claim-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.claim-checkbox input[type="checkbox"] {
  margin: 0;
}

.claim-label {
  font-weight: 500;
  color: #374151;
}

.claim-details {
  color: #6b7280;
  font-size: 0.8rem;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

.claims-loading {
  text-align: center;
  padding: 1rem;
  color: #6b7280;
  font-style: italic;
}

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.claims-count {
  background: #0366d6;
  color: white;
  border-radius: 12px;
  padding: 0.2rem 0.5rem;
  font-size: 0.75rem;
  margin-left: 0.5rem;
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  border-radius: 12px;
  max-width: 600px;
  width: 90vw;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem;
  border-bottom: 1px solid #e1e4e8;
}

.modal-header h3 {
  margin: 0;
  color: #2c3e50;
  font-size: 1.25rem;
}

.modal-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #6b7280;
  padding: 0.25rem;
}

.modal-close:hover {
  color: #374151;
}

.modal-body {
  padding: 1.5rem;
}

.modal-description {
  margin-bottom: 1.5rem;
  color: #4b5563;
}

.claims-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.claim-item {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}

.claim-header {
  padding: 1rem;
  background: #f9fafb;
}

.claim-config {
  padding: 1rem;
  border-top: 1px solid #e5e7eb;
  background: white;
}

.selector-options {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.radio-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.radio-option input[type="radio"] {
  margin: 0;
}

.label-selector-config {
  margin-top: 1rem;
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 6px;
}

.label-pairs {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.label-pair {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.label-input {
  flex: 1;
  padding: 0.5rem;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 0.875rem;
}

.label-input:focus {
  outline: none;
  border-color: #0366d6;
  box-shadow: 0 0 0 2px rgba(3, 102, 214, 0.1);
}

.btn-remove {
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  width: 30px;
  height: 30px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-remove:hover {
  background: #c82333;
}

.btn-sm {
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  padding: 1.5rem;
  border-top: 1px solid #e1e4e8;
  background: #f9fafb;
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
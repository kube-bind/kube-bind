<template>
  <div class="resources">
    <div class="header-section">
      <h2>Available Resources</h2>
      <div v-if="isCliFlow" class="cli-indicator">
        CLI Mode: Select a template to bind for CLI
      </div>
    </div>

    <div v-if="loading" class="loading">
      Loading resources...
    </div>

    <div v-else-if="error" class="error">
      <h3>Error Loading Resources</h3>
      <p>{{ error }}</p>
      <button @click="loadResources" class="retry-btn">Retry</button>
    </div>

    <div v-else class="resources-container">
      <div class="templates-section">
        <div class="section-header">
          <h3>Templates</h3>
          <span class="item-count">{{ templates.length }} available</span>
        </div>

        <div v-if="templates.length === 0" class="no-resources">
          <div class="no-resources-icon">
            <svg width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
            </svg>
          </div>
          <h4>No templates available</h4>
          <p>There are no API service export templates available in this cluster.</p>
        </div>

        <div v-else class="resource-grid">
          <div v-for="template in templates" :key="template.metadata.name" class="template-card">
            <div class="card-header">
              <h4 class="card-title">{{ template.metadata.name }}</h4>
              <div class="card-badges">
                <span v-if="template.spec.resources?.length" class="badge resources-badge">
                  {{ template.spec.resources.length }} resources
                </span>
                <span v-if="template.spec.permissionClaims?.length" class="badge permissions-badge">
                  {{ template.spec.permissionClaims.length }} permissions
                </span>
                <span v-if="template.spec.namespaces?.length" class="badge namespaces-badge">
                  {{ template.spec.namespaces.length }} namespaces
                </span>
              </div>
            </div>

            <div class="card-content">
              <p v-if="template.spec.description" class="card-description">
                {{ template.spec.description }}
              </p>

              <div v-if="template.spec.resources?.length" class="card-preview">
                <strong>Key Resources:</strong>
                <div class="resource-preview">
                  <span
                    v-for="resource in template.spec.resources.slice(0, 3)"
                    :key="resource.resource"
                    class="resource-tag"
                  >
                    {{ resource.resource }}
                  </span>
                  <span v-if="template.spec.resources.length > 3" class="more-indicator">
                    +{{ template.spec.resources.length - 3 }} more
                  </span>
                </div>
              </div>
            </div>

            <div class="card-actions">
              <button @click="showTemplateDetails(template)" class="details-btn">
                View Details
              </button>
              <button @click="openBindingModal(template)" class="bind-btn">
                {{ isCliFlow ? 'Bind for CLI' : 'Bind' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="collections.length > 0" class="collections-section">
        <div class="section-header">
          <h3>Collections</h3>
          <span class="item-count">{{ collections.length }} available</span>
        </div>

        <div class="resource-grid">
          <div v-for="collection in collections" :key="collection.metadata.name" class="collection-card">
            <div class="card-header">
              <h4 class="card-title">{{ collection.metadata.name }}</h4>
            </div>

            <div class="card-content">
              <p v-if="collection.spec.description" class="card-description">
                {{ collection.spec.description }}
              </p>

              <div v-if="collection.spec.templates?.length" class="collection-templates">
                <strong>Templates in this collection:</strong>
                <div class="template-list">
                  <span
                    v-for="templateName in collection.spec.templates.slice(0, 4)"
                    :key="templateName"
                    class="template-tag"
                  >
                    {{ templateName }}
                  </span>
                  <span v-if="collection.spec.templates.length > 4" class="more-indicator">
                    +{{ collection.spec.templates.length - 4 }} more
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Template Binding Modal -->
    <TemplateBindingModal
      v-if="selectedTemplate"
      :show="showBindingModal"
      :template="selectedTemplate"
      :is-cli-flow="isCliFlow"
      @close="closeBindingModal"
      @bind="handleBind"
    />

    <!-- Binding Result Modal -->
    <BindingResult
      v-if="bindingResponse"
      :show="showBindingResult"
      :template-name="selectedTemplateName"
      :binding-response="bindingResponse"
      @close="closeBindingResult"
    />

    <!-- Alert Modal -->
    <AlertModal
      :show="showAlert"
      :title="alertTitle"
      :message="alertMessage"
      :type="alertType"
      :preserve-whitespace="alertPreserveWhitespace"
      @close="closeAlert"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { httpClient } from '../services/http'
import type { BindableResourcesRequest, BindingResponse } from '../types/binding'
import { StructuredError } from '../services/http'
import BindingResult from '../components/BindingResult.vue'
import TemplateBindingModal from '../components/TemplateBindingModal.vue'
import AlertModal from '../components/AlertModal.vue'
import { authService } from '../services/auth'

interface Template {
  metadata: {
    name: string
  }
  spec: {
    description?: string
    resources?: Array<{
      group: string
      resource: string
      versions?: string[]
    }>
    permissionClaims?: Array<{
      group: string
      resource: string
      selector?: {
        labelSelector?: any
        namedResources?: Array<{
          name: string
          namespace?: string
        }>
        references?: Array<{
          resource: string
          group: string
          versions?: string[]
          jsonPath?: {
            name: string
            namespace?: string
          }
        }>
      }
    }>
    namespaces?: Array<{
      name: string
      description?: string
    }>
  }
}

interface Collection {
  metadata: {
    name: string
  }
  spec: {
    description?: string
    templates?: string[]
  }
}

const route = useRoute()

const loading = ref(true)
const error = ref<string | null>(null)
const templates = ref<Template[]>([])
const collections = ref<Collection[]>([])

const showBindingResult = ref(false)
const selectedTemplateName = ref('')
const bindingResponse = ref<BindingResponse | null>(null)

// Alert modal state
const showAlert = ref(false)
const alertTitle = ref('Alert')
const alertMessage = ref('')
const alertType = ref<'error' | 'warning' | 'info' | 'success'>('error')
const alertPreserveWhitespace = ref(false)
const isCliFlow = computed(() => authService.isCliFlow())

const showBindingModal = ref(false)
const selectedTemplate = ref<Template | null>(null)

const cluster = computed(() => route.query.cluster_id as string || '')
const consumerId = computed(() => route.query.consumer_id as string || '')

// Helper function to build API URLs with query parameters
const buildApiUrl = (endpoint: string) => {
  const params = new URLSearchParams()
  if (cluster.value) params.set('cluster_id', cluster.value)
  if (consumerId.value) params.set('consumer_id', consumerId.value)

  return params.toString() ? `${endpoint}?${params.toString()}` : endpoint
}

const loadResources = async () => {
  loading.value = true
  error.value = null

  try {
    const templatesUrl = buildApiUrl('/templates')
    const collectionsUrl = buildApiUrl('/collections')

    const [templatesResponse, collectionsResponse] = await Promise.all([
      httpClient.get(templatesUrl),
      httpClient.get(collectionsUrl)
    ])

    templates.value = templatesResponse.data.items || []
    collections.value = collectionsResponse.data.items || []
  } catch (err: any) {
    console.error('Failed to load resources:', err)

    // Handle structured errors
    if (err instanceof StructuredError) {
      const kubeError = err.kubeBindError

      // Don't show error for auth failures - let the HTTP interceptor handle them
      if (kubeError.code === 'AUTHENTICATION_FAILED') {
        return
      }

      // Show specific error messages based on error code
      switch (kubeError.code) {
        case 'AUTHORIZATION_FAILED':
          error.value = `Authorization failed: ${kubeError.details || kubeError.message}`
          break
        case 'CLUSTER_CONNECTION_FAILED':
          error.value = `Could not connect to cluster: ${kubeError.details || kubeError.message}`
          break
        case 'RESOURCE_NOT_FOUND':
          error.value = `Resources not found: ${kubeError.details || kubeError.message}`
          break
        default:
          error.value = `Error: ${kubeError.message}`
      }
    } else {
      // Fallback for non-structured errors
      if (err.response?.status === 401) {
        return
      }
      error.value = 'Failed to load resources. Please try again.'
    }
  } finally {
    loading.value = false
  }
}


const openBindingModal = (template: Template) => {
  selectedTemplate.value = template
  showBindingModal.value = true
}

const closeBindingModal = () => {
  showBindingModal.value = false
  selectedTemplate.value = null
}

const showTemplateDetails = (template: Template) => {
  // For now, just open the binding modal - could be extended to show details-only modal
  openBindingModal(template)
}

const handleBind = async (templateName: string, bindingName: string) => {
  try {
    const bindUrl = buildApiUrl('/bind')

    // Create the binding request
    // Use consumerId if available (CLI flow), otherwise use sessionId as cluster identity
    // Read from Vue Router's route.query instead of window.location
    const sessionIdFromRoute = route.query.session_id as string || ''
    const clusterIdentity = consumerId.value || sessionIdFromRoute

    if (!clusterIdentity) {
      showAlertModal('Missing cluster identity. Please ensure you have authenticated properly.', 'Binding Failed', 'error')
      return
    }

    const bindingRequest: BindableResourcesRequest = {
      metadata: {
        name: bindingName
      },
      spec: {
        templateRef: {
          name: templateName
        },
        clusterIdentity: {
          identity: clusterIdentity
        },
        author: 'web-ui'
      }
    }

    const response = await httpClient.post<BindingResponse>(bindUrl, bindingRequest)

    if (response.status === 200) {
      // Close the binding modal first
      closeBindingModal()

      // Check if this is a CLI flow
      if (authService.isCliFlow()) {
        console.log(response.data)
        authService.redirectToCliCallback(response.data)
      } else {
        // Show the binding result in a modal for normal UI flow
        bindingResponse.value = response.data
        selectedTemplateName.value = templateName
        showBindingResult.value = true
      }
    } else {
      showAlertModal(`Failed to bind template: ${templateName}`, 'Binding Failed', 'error')
    }
  } catch (err: any) {
    console.error('Failed to bind template:', err)

    // Handle structured errors
    if (err instanceof StructuredError) {
      const kubeError = err.kubeBindError

      // Don't show alert for auth failures - let the HTTP interceptor handle them
      if (kubeError.code === 'AUTHENTICATION_FAILED') {
        return
      }

      // Show specific error messages based on error code
      let errorMessage: string
      switch (kubeError.code) {
        case 'AUTHORIZATION_FAILED':
          errorMessage = `Authorization failed: You don't have permission to bind resources in this cluster.\n\nDetails: ${kubeError.details || kubeError.message}`
          break
        case 'CLUSTER_CONNECTION_FAILED':
          errorMessage = `Cluster connection failed: Unable to connect to the target cluster.\n\nDetails: ${kubeError.details || kubeError.message}`
          break
        case 'RESOURCE_NOT_FOUND':
          errorMessage = `Template not found: The requested template could not be found.\n\nDetails: ${kubeError.details || kubeError.message}`
          break
        default:
          errorMessage = `Failed to bind template: ${kubeError.message}\n\nDetails: ${kubeError.details || 'No additional details available'}`
      }

      showAlertModal(errorMessage, 'Binding Failed', 'error', true)
    } else {
      // Fallback for non-structured errors
      if (err.response?.status === 401) {
        return
      }
      showAlertModal(`Failed to bind template: ${templateName}. Check console for details.`, 'Binding Failed', 'error')
    }
  }
}

const closeBindingResult = () => {
  showBindingResult.value = false
  bindingResponse.value = null
  selectedTemplateName.value = ''
}

// Alert modal functions
const showAlertModal = (message: string, title = 'Error', type: 'error' | 'warning' | 'info' | 'success' = 'error', preserveWhitespace = false) => {
  alertMessage.value = message
  alertTitle.value = title
  alertType.value = type
  alertPreserveWhitespace.value = preserveWhitespace
  showAlert.value = true
}

const closeAlert = () => {
  showAlert.value = false
}

onMounted(() => {
  loadResources()
})
</script>

<style scoped>
.resources {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
}

.header-section {
  margin-bottom: 2rem;
}

.resources h2 {
  color: #1e293b;
  margin-bottom: 1rem;
  font-size: 1.875rem;
  font-weight: 600;
  font-family: inherit;
}

.resources h3 {
  color: #334155;
  font-size: 1.25rem;
  font-weight: 600;
  font-family: inherit;
  margin: 0;
}

.cli-indicator {
  background-color: #dbeafe;
  border: 1px solid #3b82f6;
  border-radius: 8px;
  padding: 0.75rem 1rem;
  color: #1e40af;
  font-weight: 500;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.loading {
  text-align: center;
  padding: 2rem;
  color: #666;
}

.error {
  text-align: center;
  padding: 2rem;
  color: #dc3545;
}

.retry-btn {
  padding: 0.5rem 1rem;
  background-color: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-top: 1rem;
}

.retry-btn:hover {
  background-color: #0056b3;
}

.resources-container {
  display: grid;
  gap: 3rem;
}

.templates-section,
.collections-section {
  background: #f8fafc;
  padding: 1.5rem;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.no-resources {
  text-align: center;
  color: #666;
  padding: 1rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.item-count {
  font-size: 0.9rem;
  color: #666;
  background: #e9ecef;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-weight: 500;
}

.no-resources {
  text-align: center;
  padding: 3rem 2rem;
  color: #6b7280;
  background: white;
  border-radius: 12px;
  border: 2px dashed #d1d5db;
}

.no-resources-icon {
  margin-bottom: 1rem;
  opacity: 0.4;
  color: #9ca3af;
}

.no-resources h4 {
  margin: 0 0 0.5rem 0;
  color: #374151;
  font-weight: 600;
}

.no-resources p {
  margin: 0;
  font-size: 0.9rem;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 1.5rem;
}

.template-card, .collection-card {
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 0;
  transition: all 0.2s ease;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.template-card:hover, .collection-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
  border-color: #cbd5e1;
}

.card-header {
  padding: 1.5rem 1.5rem 1rem 1.5rem;
  border-bottom: 1px solid #f1f5f9;
}

.card-title {
  margin: 0 0 0.75rem 0;
  color: #1e293b;
  font-size: 1.125rem;
  font-weight: 600;
}

.card-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.badge {
  font-size: 0.75rem;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
}

.resources-badge {
  background: #dbeafe;
  color: #1d4ed8;
}

.permissions-badge {
  background: #fef3c7;
  color: #d97706;
}

.namespaces-badge {
  background: #d1fae5;
  color: #047857;
}

.card-content {
  padding: 1rem 1.5rem;
}

.card-description {
  color: #64748b;
  margin: 0 0 1rem 0;
  line-height: 1.5;
  font-size: 0.9rem;
}

.card-preview, .collection-templates {
  margin-top: 1rem;
}

.card-preview strong, .collection-templates strong {
  color: #374151;
  font-size: 0.875rem;
  display: block;
  margin-bottom: 0.5rem;
}

.resource-preview, .template-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.resource-tag, .template-tag {
  font-size: 0.75rem;
  background: #f1f5f9;
  color: #475569;
  padding: 0.25rem 0.5rem;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}

.more-indicator {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
}

.card-actions {
  padding: 1rem 1.5rem;
  background: #fafbfc;
  border-top: 1px solid #f1f5f9;
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}

.details-btn, .bind-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
  font-size: 0.875rem;
  transition: all 0.2s ease;
}

.details-btn {
  background: #f8fafc;
  color: #475569;
  border: 1px solid #e2e8f0;
}

.details-btn:hover {
  background: #f1f5f9;
  border-color: #cbd5e1;
}

.bind-btn {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: white;
  border: 1px solid transparent;
}

.bind-btn:hover {
  background: linear-gradient(135deg, #059669 0%, #047857 100%);
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(16, 185, 129, 0.2);
}

</style>

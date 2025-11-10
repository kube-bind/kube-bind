<template>
  <div v-if="show" class="modal-overlay" @click="closeModal">
    <div class="modal" @click.stop>
      <div class="modal-header">
        <h3>Bind Template: {{ template.metadata.name }}</h3>
        <button @click="closeModal" class="close-btn">&times;</button>
      </div>
      
      <div class="modal-content">
        <!-- Binding Name Section -->
        <div class="binding-name-section">
          <label for="bindingName" class="form-label">Binding Name</label>
          <input
            id="bindingName"
            v-model="bindingName"
            type="text"
            class="form-input"
            :class="{ 'invalid': !isValidBindingName }"
            placeholder="Enter a unique name for this binding"
            @keyup.enter="handleBind"
          />
          <p v-if="isValidBindingName" class="form-help">This name will be used to identify your binding in the CLI.</p>
          <p v-else class="form-error">Name must be lowercase letters, numbers, and hyphens only. Must start and end with alphanumeric characters.</p>
        </div>

        <!-- Template Details -->
        <div class="template-details">
          <h4>Template Details</h4>
          
          <div v-if="template.spec.description" class="detail-section">
            <h5>Description</h5>
            <p class="description">{{ template.spec.description }}</p>
          </div>

          <!-- Resources -->
          <div v-if="template.spec.resources && template.spec.resources.length > 0" class="detail-section">
            <h5>Resources ({{ template.spec.resources.length }})</h5>
            <div class="resource-list">
              <div v-for="resource in template.spec.resources" :key="`${resource.group}/${resource.resource}`" class="resource-item">
                <span class="resource-name">{{ resource.resource }}</span>
                <span class="resource-group">{{ resource.group || 'core' }}</span>
                <span v-if="resource.versions" class="resource-versions">
                  {{ resource.versions.join(', v') }}
                </span>
              </div>
            </div>
          </div>

          <!-- Permission Claims -->
          <div v-if="template.spec.permissionClaims && template.spec.permissionClaims.length > 0" class="detail-section">
            <h5>Permission Claims ({{ template.spec.permissionClaims.length }})</h5>
            <div class="permission-list">
              <div v-for="claim in template.spec.permissionClaims" :key="`${claim.group}/${claim.resource}`" class="permission-item">
                <span class="permission-name">{{ claim.resource }}</span>
                <span class="permission-group">{{ claim.group || 'core' }}</span>
                <div v-if="claim.selector" class="permission-selector">
                  <span v-if="claim.selector.labelSelector" class="selector-labels">
                    Labels: {{ formatLabelSelector(claim.selector.labelSelector) }}
                  </span>
                  <span v-if="claim.selector.namedResources && claim.selector.namedResources.length > 0" class="selector-names">
                    Named: {{ formatNamedResources(claim.selector.namedResources) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Namespaces -->
          <div v-if="template.spec.namespaces && template.spec.namespaces.length > 0" class="detail-section">
            <h5>Namespaces ({{ template.spec.namespaces.length }})</h5>
            <div class="namespace-list">
              <div v-for="ns in template.spec.namespaces" :key="ns.name" class="namespace-item">
                <span class="namespace-name">{{ ns.name }}</span>
                <span v-if="ns.description" class="namespace-desc">{{ ns.description }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div class="modal-footer">
        <button @click="closeModal" class="cancel-btn">Cancel</button>
        <button @click="handleBind" :disabled="!bindingName.trim() || binding || !isValidBindingName" class="bind-btn">
          <span v-if="binding">Binding...</span>
          <span v-else>{{ isCliFlow ? 'Bind for CLI' : 'Bind Template' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

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
        namedResources?: string[]
      }
    }>
    namespaces?: Array<{
      name: string
      description?: string
    }>
  }
}

interface Props {
  show: boolean
  template: Template
  isCliFlow: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  bind: [templateName: string, bindingName: string]
}>()

const binding = ref(false)
const bindingName = ref('')

// Validate binding name according to Kubernetes naming conventions
const isValidBindingName = computed(() => {
  const name = bindingName.value.trim()
  if (!name) return false
  
  // Kubernetes RFC 1123 subdomain rules: lowercase alphanumeric, hyphens, dots
  // Must start and end with alphanumeric character
  const k8sNameRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/
  return k8sNameRegex.test(name)
})

// Auto-generate default binding name when template changes
watch(() => props.template, (newTemplate) => {
  if (newTemplate?.metadata?.name) {
    // Generate Kubernetes-compliant name: lowercase alphanumeric and hyphens only
    const templateName = newTemplate.metadata.name.toLowerCase().replace(/[^a-z0-9-]/g, '-')
    bindingName.value = templateName
  }
}, { immediate: true })

const closeModal = () => {
  if (!binding.value) {
    emit('close')
  }
}

const handleBind = async () => {
  const name = bindingName.value.trim()
  if (!name || binding.value || !isValidBindingName.value) return
  
  binding.value = true
  try {
    emit('bind', props.template.metadata.name, name)
  } finally {
    binding.value = false
  }
}

const formatLabelSelector = (selector: any): string => {
  if (!selector) return ''
  if (typeof selector === 'string') return selector
  if (selector.matchLabels) {
    return Object.entries(selector.matchLabels)
      .map(([key, value]) => `${key}=${value}`)
      .join(', ')
  }
  return JSON.stringify(selector)
}

const formatNamedResources = (namedResources: any[]): string => {
  if (!namedResources || namedResources.length === 0) return ''
  
  return namedResources.map(resource => {
    if (typeof resource === 'string') return resource
    if (typeof resource === 'object' && resource.name) return resource.name
    if (typeof resource === 'object' && resource.namespace && resource.name) {
      return `${resource.namespace}/${resource.name}`
    }
    return JSON.stringify(resource)
  }).join(', ')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.2s ease;
}

.modal {
  background: white;
  border-radius: 12px;
  width: 90%;
  max-width: 700px;
  max-height: 85vh;
  overflow: hidden;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  animation: slideIn 0.3s ease;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes slideIn {
  from { transform: translateY(-20px) scale(0.95); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  border-bottom: 1px solid #e5e7eb;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: rgba(255, 255, 255, 0.8);
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.modal-content {
  padding: 2rem;
  max-height: 60vh;
  overflow-y: auto;
}

.binding-name-section {
  margin-bottom: 2rem;
  padding-bottom: 2rem;
  border-bottom: 1px solid #e5e7eb;
}

.form-label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 600;
  color: #374151;
}

.form-input {
  width: 100%;
  padding: 0.75rem 1rem;
  border: 2px solid #d1d5db;
  border-radius: 8px;
  font-size: 1rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-input:focus {
  outline: none;
  border-color: #667eea;
  box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
}

.form-help {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
}

.form-input.invalid {
  border-color: #dc2626;
  background-color: #fef2f2;
}

.form-input.invalid:focus {
  border-color: #dc2626;
  box-shadow: 0 0 0 3px rgba(220, 38, 38, 0.1);
}

.form-error {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: #dc2626;
  font-weight: 500;
}

.template-details h4 {
  margin-bottom: 1.5rem;
  color: #111827;
  font-size: 1.125rem;
  font-weight: 600;
}

.detail-section {
  margin-bottom: 2rem;
}

.detail-section h5 {
  margin-bottom: 1rem;
  color: #374151;
  font-size: 1rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.description {
  color: #6b7280;
  line-height: 1.6;
  background: #f9fafb;
  padding: 1rem;
  border-radius: 8px;
  border-left: 4px solid #667eea;
}

.resource-list, .permission-list, .namespace-list {
  display: grid;
  gap: 0.75rem;
}

.resource-item, .permission-item, .namespace-item {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 1rem;
  transition: border-color 0.2s;
}

.resource-item:hover, .permission-item:hover, .namespace-item:hover {
  border-color: #cbd5e1;
}

.resource-name, .permission-name, .namespace-name {
  font-weight: 600;
  color: #1e293b;
  display: block;
  margin-bottom: 0.25rem;
}

.resource-group, .permission-group {
  font-size: 0.875rem;
  color: #64748b;
  background: #e2e8f0;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  display: inline-block;
  margin-right: 0.5rem;
}

.resource-versions {
  font-size: 0.875rem;
  color: #059669;
  background: #d1fae5;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  display: inline-block;
}

.permission-selector {
  margin-top: 0.5rem;
  font-size: 0.875rem;
}

.selector-labels, .selector-names {
  display: block;
  color: #6b7280;
  margin-bottom: 0.25rem;
}

.namespace-desc {
  font-size: 0.875rem;
  color: #6b7280;
  display: block;
  margin-top: 0.25rem;
}

.modal-footer {
  padding: 1.5rem 2rem;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
}

.cancel-btn, .bind-btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s;
}

.cancel-btn {
  background: #f3f4f6;
  color: #374151;
}

.cancel-btn:hover {
  background: #e5e7eb;
}

.bind-btn {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: white;
}

.bind-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #059669 0%, #047857 100%);
  transform: translateY(-1px);
}

.bind-btn:disabled {
  background: #d1d5db;
  color: #9ca3af;
  cursor: not-allowed;
  transform: none;
}
</style>
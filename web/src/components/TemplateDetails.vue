<template>
  <div v-if="show" class="modal-overlay" @click.self="$emit('close')">
    <div class="modal-container">
      <div class="modal-header">
        <h3>{{ template.metadata.name }}</h3>
        <button class="close-btn" @click="$emit('close')">&times;</button>
      </div>

      <div class="modal-body">
        <!-- Template Info -->
        <section class="detail-section">
          <h4>Resources</h4>
          <div v-if="template.spec.resources?.length" class="tag-list">
            <span
              v-for="r in template.spec.resources"
              :key="r.resource"
              class="tag"
            >
              {{ r.group ? `${r.resource}.${r.group}` : r.resource }}
            </span>
          </div>
          <p v-else class="empty-note">No resources defined.</p>
        </section>

        <section v-if="template.spec.permissionClaims?.length" class="detail-section">
          <h4>Permission Claims</h4>
          <div class="tag-list">
            <span
              v-for="(claim, i) in template.spec.permissionClaims"
              :key="i"
              class="tag permission-tag"
            >
              {{ claim.group ? `${claim.resource}.${claim.group}` : claim.resource }}
            </span>
          </div>
        </section>

        <section v-if="template.spec.description" class="detail-section">
          <h4>Description</h4>
          <p>{{ template.spec.description }}</p>
        </section>

        <!-- Active Service Exports -->
        <section class="detail-section exports-section">
          <h4>Active Service Exports</h4>
          <div v-if="statusLoading" class="status-loading">
            <span class="spinner"></span> Checking connection&hellip;
          </div>
          <div v-else-if="status?.connected && status.exports.length > 0" class="exports-list">
            <div class="connection-info">
              <span class="connected-badge">Connected</span>
              <span class="ns-label">Namespace: <strong>{{ status.namespace }}</strong></span>
            </div>
            <div class="tag-list">
              <span v-for="name in status.exports" :key="name" class="tag export-tag">
                {{ name }}
              </span>
            </div>
          </div>
          <div v-else class="empty-note">
            No active exports. Bind this template to create service exports on the provider.
          </div>
        </section>
      </div>

      <div class="modal-footer">
        <button class="secondary-btn" @click="$emit('close')">Close</button>
        <button class="primary-btn" @click="$emit('bind', template)">Bind</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Template {
  metadata: { name: string }
  spec: {
    description?: string
    resources?: Array<{ group: string; resource: string; versions?: string[] }>
    permissionClaims?: Array<{ group: string; resource: string }>
    namespaces?: Array<{ name: string; description?: string }>
  }
}

interface ConsumerStatusResponse {
  connected: boolean
  namespace?: string
  exports: string[]
}

const props = defineProps<{
  show: boolean
  template: Template
}>()

defineEmits<{
  close: []
  bind: [template: Template]
}>()

const status = ref<ConsumerStatusResponse | null>(null)
const statusLoading = ref(false)

const fetchStatus = async () => {
  statusLoading.value = true
  try {
    const response = await fetch('/api/consumer-status')
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    status.value = await response.json()
  } catch (err) {
    console.error('Failed to check consumer status:', err)
    status.value = null
  } finally {
    statusLoading.value = false
  }
}

watch(() => props.show, (visible) => {
  if (visible) {
    status.value = null
    fetchStatus()
  }
})
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-container {
  background: white;
  border-radius: 12px;
  width: 560px;
  max-width: 90vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid #e2e8f0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  color: #1e293b;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: #64748b;
  cursor: pointer;
  line-height: 1;
}

.modal-body {
  padding: 1.5rem;
  overflow-y: auto;
  flex: 1;
}

.detail-section {
  margin-bottom: 1.25rem;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.detail-section h4 {
  margin: 0 0 0.5rem;
  font-size: 0.875rem;
  color: #475569;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.tag {
  padding: 0.25rem 0.625rem;
  border-radius: 4px;
  font-size: 0.8rem;
  font-family: 'SF Mono', Monaco, monospace;
  background: #f1f5f9;
  color: #334155;
  border: 1px solid #e2e8f0;
}

.permission-tag {
  background: #fef3c7;
  border-color: #fbbf24;
  color: #92400e;
}

.export-tag {
  background: #ecfdf5;
  border-color: #6ee7b7;
  color: #065f46;
}

.empty-note {
  color: #94a3b8;
  font-size: 0.875rem;
  font-style: italic;
  margin: 0;
}

.exports-section {
  border-top: 1px solid #e2e8f0;
  padding-top: 1.25rem;
}

.connection-info {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.625rem;
}

.connected-badge {
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.7rem;
  text-transform: uppercase;
  background: #22c55e;
  color: white;
}

.ns-label {
  font-size: 0.8rem;
  color: #475569;
}

.status-loading {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: #64748b;
  font-size: 0.875rem;
}

.spinner {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid #e2e8f0;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  border-top: 1px solid #e2e8f0;
}

.secondary-btn {
  padding: 0.5rem 1rem;
  border-radius: 6px;
  border: 1px solid #cbd5e1;
  background: white;
  color: #475569;
  cursor: pointer;
  font-size: 0.875rem;
}

.secondary-btn:hover {
  background: #f8fafc;
}

.primary-btn {
  padding: 0.5rem 1.25rem;
  border-radius: 6px;
  border: none;
  background: #3b82f6;
  color: white;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
}

.primary-btn:hover {
  background: #2563eb;
}
</style>

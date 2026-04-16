<template>
  <div v-if="show" class="binding-modal-overlay" @click="closeModal">
    <div class="binding-modal" @click.stop>
      <div class="binding-header">
        <h3>Template Binding Successful</h3>
        <button @click="closeModal" class="close-btn">&times;</button>
      </div>

      <div class="binding-content">
        <div class="binding-info">
          <h4>Binding Information</h4>
          <p><strong>Template:</strong> {{ templateName }}</p>
          <p><strong>Binding Name:</strong> {{ bindingName }}</p>
        </div>

        <!-- Method selector tabs -->
        <div class="method-tabs">
          <button
            :class="['method-tab', { active: activeMethod === 'oneclick' }]"
            @click="activeMethod = 'oneclick'"
          >
            ⚡ One-Click
          </button>
          <button
            :class="['method-tab', { active: activeMethod === 'connected' }]"
            @click="activeMethod = 'connected'"
          >
            🔗 Already Connected
          </button>
          <button
            :class="['method-tab', { active: activeMethod === 'bundle' }]"
            @click="activeMethod = 'bundle'"
          >
            📦 Bundle
          </button>
          <button
            :class="['method-tab', { active: activeMethod === 'manual' }]"
            @click="activeMethod = 'manual'"
          >
            🔧 Manual (CLI)
          </button>
        </div>

        <!-- Method: One-Click (Upload Kubeconfig) -->
        <div v-if="activeMethod === 'oneclick'" class="instructions-section">
          <p class="instructions-text">
            Upload your consumer cluster kubeconfig and we'll automatically deploy the konnector agent
            and configure the binding bundle for you. No CLI required.
          </p>

          <div class="security-warning">
            <strong>⚠️ Security Note:</strong> Your kubeconfig will be used transiently to apply resources
            to your consumer cluster and will <strong>not</strong> be stored. The provider backend needs
            cluster-admin level access to deploy the konnector and create binding resources.
          </div>

          <div class="step-group">
            <h5>Upload Consumer Kubeconfig</h5>
            <p class="step-description">Select the kubeconfig file for your consumer cluster.</p>
            <div class="upload-block">
              <input
                type="file"
                ref="kubeconfigFileInput"
                accept=".yaml,.yml,.conf,.kubeconfig"
                @change="handleKubeconfigUpload"
                class="file-input"
                :disabled="applyStatus === 'loading'"
              />
              <div v-if="kubeconfigFileName" class="file-info">
                📄 {{ kubeconfigFileName }}
              </div>
            </div>
          </div>

          <div class="step-group">
            <button
              @click="applyToConsumer"
              class="apply-btn"
              :disabled="!kubeconfigData || applyStatus === 'loading'"
            >
              <span v-if="applyStatus === 'loading'" class="spinner"></span>
              {{ applyStatus === 'loading' ? 'Applying...' : 'Apply to Consumer Cluster' }}
            </button>
          </div>

          <div v-if="applyStatus === 'success'" class="status-message success">
            ✅ {{ applyMessage }}
          </div>
          <div v-if="applyStatus === 'error'" class="status-message error">
            ❌ {{ applyMessage }}
          </div>
        </div>

        <!-- Method: Already Connected -->
        <div v-if="activeMethod === 'connected'" class="instructions-section">
          <div v-if="consumerStatusLoading" class="status-check">
            <span class="spinner"></span> Checking consumer connection status...
          </div>

          <div v-else-if="consumerStatus?.connected" class="connected-info">
            <div class="connected-badge">✅ Consumer Cluster Connected</div>
            <p class="instructions-text">
              Your consumer cluster already has a konnector agent with an active binding bundle.
              The new service <strong>{{ templateName }}</strong> has been registered on the provider
              and will be automatically discovered by your konnector within ~15 seconds.
            </p>
            <div v-if="consumerStatus.exports.length > 0" class="exports-list">
              <h5>Active Service Exports</h5>
              <ul>
                <li v-for="exp in consumerStatus.exports" :key="exp">{{ exp }}</li>
              </ul>
            </div>
            <div class="step-group">
              <h5>Verify on your consumer cluster</h5>
              <div class="command-block">
                <code>kubectl get apiservicebindingbundles,apiservicebindings</code>
                <button @click="copyCommand('kubectl get apiservicebindingbundles,apiservicebindings')" class="copy-cmd-btn">Copy</button>
              </div>
            </div>
          </div>

          <div v-else class="not-connected-info">
            <div class="not-connected-badge">ℹ️ No Connected Consumer Found</div>
            <p class="instructions-text">
              No existing consumer cluster was found for your identity. Use one of the other methods
              to set up the initial connection:
            </p>
            <ul class="method-suggestions">
              <li><strong>One-Click</strong> — Upload your consumer kubeconfig for automatic setup</li>
              <li><strong>Bundle</strong> — Download and apply manifests manually</li>
              <li><strong>Manual (CLI)</strong> — Use kubectl bind apiservice</li>
            </ul>
          </div>
        </div>

        <!-- Method: Automated via APIServiceBindingBundle -->
        <div v-if="activeMethod === 'bundle'" class="instructions-section">
          <p class="instructions-text">
            Download and apply these files to your consumer cluster.
            The APIServiceBindingBundle will automatically discover and bind services from the provider.
          </p>

          <div class="step-group">
            <h5>1. Deploy the konnector agent (one-time)</h5>
            <p class="step-description">Skip this step if the konnector is already deployed on your consumer cluster.</p>
            <div class="download-block">
              <div class="download-actions">
                <button @click="downloadKonnectorManifests" class="download-btn primary">Download konnector.yaml</button>
              </div>
            </div>
            <div class="command-block">
              <code>kubectl apply -f konnector.yaml</code>
              <button @click="copyCommand('kubectl apply -f konnector.yaml')" class="copy-cmd-btn">Copy</button>
            </div>
          </div>

          <div class="step-group">
            <h5>2. Apply the binding bundle</h5>
            <p class="step-description">Creates the kubeconfig secret and an APIServiceBindingBundle that auto-discovers and binds all services.</p>
            <div class="download-block">
              <div class="download-actions">
                <button @click="downloadBindingBundle" class="download-btn primary">Download binding-bundle.yaml</button>
              </div>
            </div>
            <div class="command-block">
              <code>kubectl apply -f binding-bundle.yaml</code>
              <button @click="copyCommand('kubectl apply -f binding-bundle.yaml')" class="copy-cmd-btn">Copy</button>
            </div>
          </div>

          <div class="step-group">
            <h5>3. Verify</h5>
            <div class="command-block">
              <code>kubectl get apiservicebindingbundles,apiservicebindings</code>
              <button @click="copyCommand('kubectl get apiservicebindingbundles,apiservicebindings')" class="copy-cmd-btn">Copy</button>
            </div>
          </div>
        </div>

        <!-- Method: Manual via kubectl bind apiservice -->
        <div v-if="activeMethod === 'manual'" class="instructions-section">
          <p class="instructions-text">
            Download the required files and run the following commands on your consumer cluster.
          </p>

          <div class="step-group">
            <h5>1. Download required files</h5>
            <div class="download-block">
              <div class="download-actions">
                <button @click="downloadKubeconfig" class="download-btn primary">Download kubeconfig.yaml</button>
                <button @click="downloadAPIRequests" class="download-btn primary">Download apiservice-export.yaml</button>
              </div>
            </div>
          </div>

          <div class="step-group">
            <h5>2. Create kube-bind namespace</h5>
            <div class="command-block">
              <code>kubectl create namespace kube-bind --dry-run=client -o yaml | kubectl apply -f -</code>
              <button @click="copyCommand('kubectl create namespace kube-bind --dry-run=client -o yaml | kubectl apply -f -')" class="copy-cmd-btn">Copy</button>
            </div>
          </div>

          <div class="step-group">
            <h5>3. Create kubeconfig secret</h5>
            <div class="command-block">
              <code>{{ createSecretCommand }}</code>
              <button @click="copyCommand(createSecretCommand)" class="copy-cmd-btn">Copy</button>
            </div>
          </div>

          <div class="step-group">
            <h5>4. Bind the API service</h5>
            <div class="command-block">
              <code>{{ bindCommand }}</code>
              <button @click="copyCommand(bindCommand)" class="copy-cmd-btn">Copy</button>
            </div>
          </div>
        </div>
      </div>

      <div class="binding-footer">
        <button @click="closeModal" class="ok-btn">Close</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { BindingResponse } from '../types/binding'

interface ConsumerStatusResponse {
  connected: boolean
  namespace?: string
  exports: string[]
}

interface ApplyResult {
  konnectorDeployed: boolean
  bundleCreated: boolean
  message: string
}

interface Props {
  show: boolean
  templateName: string
  bindingResponse: BindingResponse
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
}>()

const activeMethod = ref<'oneclick' | 'connected' | 'bundle' | 'manual'>('oneclick')

// One-Click state
const kubeconfigData = ref<string | null>(null)
const kubeconfigFileName = ref<string>('')
const applyStatus = ref<'idle' | 'loading' | 'success' | 'error'>('idle')
const applyMessage = ref('')
const kubeconfigFileInput = ref<HTMLInputElement | null>(null)

// Already Connected state
const consumerStatus = ref<ConsumerStatusResponse | null>(null)
const consumerStatusLoading = ref(false)

const bindingName = computed(() => {
  return props.bindingResponse.bindingName || props.templateName
})

const kubeconfigSecretName = computed(() => {
  const safeName = bindingName.value.toLowerCase().replace(/[^a-z0-9-]/g, '-')
  return `kubeconfig-${safeName}`
})

// Manual method commands
const createSecretCommand = computed(() => {
  return `kubectl create secret generic ${kubeconfigSecretName.value} --from-file=kubeconfig=./kubeconfig.yaml -n kube-bind`
})

const bindCommand = computed(() => {
  return `kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name ${kubeconfigSecretName.value} -f apiservice-export.yaml`
})

// Check consumer status when "Already Connected" tab is selected
watch(activeMethod, async (method) => {
  if (method === 'connected' && !consumerStatus.value) {
    await checkConsumerStatus()
  }
})

const checkConsumerStatus = async () => {
  consumerStatusLoading.value = true
  try {
    const response = await fetch('/api/consumer-status')
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    consumerStatus.value = await response.json()
  } catch (error) {
    console.error('Failed to check consumer status:', error)
    consumerStatus.value = { connected: false, exports: [] }
  } finally {
    consumerStatusLoading.value = false
  }
}

// One-Click: handle kubeconfig file upload
const handleKubeconfigUpload = (event: Event) => {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  kubeconfigFileName.value = file.name
  applyStatus.value = 'idle'
  applyMessage.value = ''

  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target?.result as string
    // Base64 encode the kubeconfig content
    kubeconfigData.value = btoa(content)
  }
  reader.readAsText(file)
}

// One-Click: apply binding to consumer cluster
const applyToConsumer = async () => {
  if (!kubeconfigData.value) return

  applyStatus.value = 'loading'
  applyMessage.value = ''

  try {
    const response = await fetch('/api/apply-binding', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        consumerKubeconfig: kubeconfigData.value,
        bindingName: bindingName.value,
      }),
    })

    if (!response.ok) {
      const errorData = await response.json().catch(() => null)
      throw new Error(errorData?.message || `HTTP ${response.status}`)
    }

    const result: ApplyResult = await response.json()
    applyStatus.value = 'success'
    applyMessage.value = result.message
    if (result.konnectorDeployed) {
      applyMessage.value += ' (konnector was newly deployed)'
    }
  } catch (error: any) {
    applyStatus.value = 'error'
    applyMessage.value = error.message || 'Failed to apply binding'
  }
}

const closeModal = () => {
  emit('close')
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
  } catch (err) {
    console.error('Failed to copy command:', err)
    const textarea = document.createElement('textarea')
    textarea.value = command
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}

const triggerDownload = (content: string, filename: string, type = 'text/yaml') => {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// Decode kubeconfig from base64
const decodedKubeconfig = computed(() => {
  try {
    return atob(props.bindingResponse.kubeconfig)
  } catch {
    return props.bindingResponse.kubeconfig
  }
})

// Bundle method: binding-bundle.yaml
const bindingBundleYaml = computed(() => {
  const kubeconfigBase64 = props.bindingResponse.kubeconfig
  const secretName = kubeconfigSecretName.value
  const bundleName = bindingName.value

  return `apiVersion: v1
kind: Namespace
metadata:
  name: kube-bind
---
apiVersion: v1
kind: Secret
metadata:
  name: ${secretName}
  namespace: kube-bind
type: Opaque
data:
  kubeconfig: ${kubeconfigBase64}
---
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: ${bundleName}
spec:
  kubeconfigSecretRef:
    name: ${secretName}
    namespace: kube-bind
    key: kubeconfig
`
})

const downloadBindingBundle = () => {
  triggerDownload(bindingBundleYaml.value, 'binding-bundle.yaml')
}

const downloadKonnectorManifests = async () => {
  try {
    const response = await fetch('/api/konnector-manifests')
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const yaml = await response.text()
    triggerDownload(yaml, 'konnector.yaml')
  } catch (error) {
    console.error('Failed to fetch konnector manifests:', error)
  }
}

// Manual method: individual file downloads
const downloadKubeconfig = () => {
  triggerDownload(decodedKubeconfig.value, 'kubeconfig.yaml')
}

const downloadAPIRequests = () => {
  try {
    const apiRequestsYaml = props.bindingResponse.requests.map(req => {
      if (typeof req === 'string') {
        return req.trim()
      } else {
        return JSON.stringify(req, null, 2)
      }
    }).join('\n---\n')

    triggerDownload(apiRequestsYaml, 'apiservice-export.yaml')
  } catch (error) {
    console.error('Failed to format API requests:', error)
    const json = JSON.stringify(props.bindingResponse.requests, null, 2)
    triggerDownload(json, 'apiservice-export.json', 'application/json')
  }
}
</script>

<style scoped>
.binding-modal-overlay {
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
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
}

.binding-modal {
  background: white;
  border-radius: 12px;
  width: 90%;
  max-width: 900px;
  max-height: 85vh;
  overflow: hidden;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.binding-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  border-bottom: 1px solid #e5e7eb;
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: white;
}

.binding-header h3 {
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

.binding-content {
  padding: 2rem;
  max-height: 70vh;
  overflow-y: auto;
}

.binding-info {
  margin-bottom: 1.5rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid #e5e7eb;
}

.binding-info h4 {
  margin-bottom: 1rem;
  color: #111827;
  font-size: 1.125rem;
  font-weight: 600;
}

.binding-info p {
  margin: 0.5rem 0;
  color: #6b7280;
  font-size: 0.9rem;
}

.method-tabs {
  display: flex;
  gap: 0;
  margin-bottom: 1.5rem;
  border-bottom: 2px solid #e5e7eb;
  flex-wrap: wrap;
}

.method-tab {
  padding: 0.75rem 1.25rem;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  cursor: pointer;
  font-size: 0.85rem;
  font-weight: 500;
  color: #6b7280;
  transition: all 0.2s;
  white-space: nowrap;
}

.method-tab:hover {
  color: #374151;
}

.method-tab.active {
  color: #3b82f6;
  border-bottom-color: #3b82f6;
}

.instructions-section {
  margin-bottom: 1rem;
}

.instructions-text {
  margin-bottom: 1.5rem;
  color: #6b7280;
  line-height: 1.6;
}

.step-group {
  margin-bottom: 2rem;
}

.step-group h5 {
  margin-bottom: 0.5rem;
  color: #374151;
  font-size: 1rem;
  font-weight: 600;
}

.step-description {
  margin-bottom: 0.75rem;
  color: #6b7280;
  font-size: 0.875rem;
}

.download-block {
  background: #f0f9ff;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 0.75rem;
}

.download-actions {
  display: flex;
  gap: 1rem;
}

.download-btn {
  padding: 0.75rem 1.5rem;
  background: #6b7280;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 0.2s;
}

.download-btn:hover {
  background: #4b5563;
}

.download-btn.primary {
  background: #3b82f6;
}

.download-btn.primary:hover {
  background: #2563eb;
}

.command-block {
  display: flex;
  align-items: center;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 1rem;
  gap: 1rem;
}

.command-block code {
  flex: 1;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', 'Consolas', monospace;
  font-size: 0.875rem;
  color: #1f2937;
  background: none;
  word-break: break-all;
}

.copy-cmd-btn {
  padding: 0.5rem 1rem;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 0.2s;
  flex-shrink: 0;
}

.copy-cmd-btn:hover {
  background: #2563eb;
}

/* One-Click tab styles */
.security-warning {
  background: #fffbeb;
  border: 1px solid #fcd34d;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1.5rem;
  color: #92400e;
  font-size: 0.875rem;
  line-height: 1.5;
}

.upload-block {
  background: #f9fafb;
  border: 2px dashed #d1d5db;
  border-radius: 8px;
  padding: 1.5rem;
  text-align: center;
}

.file-input {
  margin-bottom: 0.5rem;
}

.file-info {
  margin-top: 0.5rem;
  color: #059669;
  font-size: 0.875rem;
  font-weight: 500;
}

.apply-btn {
  padding: 0.875rem 2rem;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 600;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.apply-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  transform: translateY(-1px);
}

.apply-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 50%;
  border-top-color: white;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.status-message {
  padding: 1rem;
  border-radius: 8px;
  font-size: 0.9rem;
  margin-top: 1rem;
}

.status-message.success {
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
}

.status-message.error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

/* Already Connected tab styles */
.status-check {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem;
  justify-content: center;
  color: #6b7280;
}

.status-check .spinner {
  border-color: rgba(59, 130, 246, 0.3);
  border-top-color: #3b82f6;
}

.connected-badge {
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 1rem;
  margin-bottom: 1rem;
}

.not-connected-badge {
  background: #f0f9ff;
  border: 1px solid #bfdbfe;
  color: #1e40af;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 1rem;
  margin-bottom: 1rem;
}

.exports-list {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1.5rem;
}

.exports-list h5 {
  margin: 0 0 0.5rem 0;
  color: #374151;
  font-size: 0.9rem;
}

.exports-list ul {
  margin: 0;
  padding-left: 1.5rem;
}

.exports-list li {
  color: #6b7280;
  font-size: 0.875rem;
  font-family: 'SF Mono', 'Monaco', monospace;
  padding: 0.25rem 0;
}

.method-suggestions {
  padding-left: 1.5rem;
  color: #6b7280;
  line-height: 2;
}

.binding-footer {
  padding: 1.5rem 2rem;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
  text-align: right;
}

.ok-btn {
  padding: 0.75rem 1.5rem;
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s;
}

.ok-btn:hover {
  background: linear-gradient(135deg, #059669 0%, #047857 100%);
  transform: translateY(-1px);
}
</style>
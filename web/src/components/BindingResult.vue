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
          <p><strong>Kubeconfig Secret:</strong> {{ kubeconfigSecretName }}</p>
        </div>
        
        <div class="instructions-section">
          <h4>Setup Instructions</h4>
          <p class="instructions-text">
            To complete the binding setup, first download the required files below, then execute the following commands in your local kubectl environment:
          </p>
          
          <div class="download-files-section">
            <h5>1. Download required files</h5>
            <div class="download-block">
              <p class="download-text">Download and save both files in your current directory:</p>
              <div class="download-actions">
                <button @click="downloadKubeconfig" class="download-btn">Download kubeconfig.yaml</button>
                <button @click="downloadAPIRequests" class="download-btn">Download apiservice-export.yaml</button>
              </div>
            </div>
          </div>
          
          <div class="command-group">
            <h5>2. Create kube-bind namespace (if it doesn't exist)</h5>
            <div class="command-block">
              <code>kubectl create namespace kube-bind --dry-run=client -o yaml | kubectl apply -f -</code>
              <button @click="copyCommand('kubectl create namespace kube-bind --dry-run=client -o yaml | kubectl apply -f -')" class="copy-cmd-btn">Copy</button>
            </div>
          </div>
          
          <div class="command-group">
            <h5>3. Create kubeconfig secret</h5>
            <div class="command-block">
              <code>{{ createSecretCommand }}</code>
              <button @click="copyCommand(createSecretCommand)" class="copy-cmd-btn">Copy</button>
            </div>
          </div>
          
          <div class="command-group">
            <h5>4. Bind the API service</h5>
            <div class="command-block">
              <code>{{ bindCommand }}</code>
              <button @click="copyCommand(bindCommand)" class="copy-cmd-btn">Copy</button>
            </div>
          </div>
        </div>
        
        <div class="alternative-section">
          <details>
            <summary>Alternative: Use stdin piping</summary>
            <div class="manual-setup">
              <h5>For advanced users who prefer piping:</h5>
              <div class="command-group">
                <div class="command-block">
                  <code>cat apiservice-export.yaml | kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name {{ kubeconfigSecretName }} -f -</code>
                  <button @click="copyCommand(`cat apiservice-export.yaml | kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name ${kubeconfigSecretName} -f -`)" class="copy-cmd-btn">Copy</button>
                </div>
              </div>
            </div>
          </details>
        </div>
      </div>
      
      <div class="binding-footer">
        <button @click="closeModal" class="ok-btn">Close</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { BindingResponse } from '../types/binding'

interface Props {
  show: boolean
  templateName: string
  bindingResponse: BindingResponse
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
}>()

// Generate a secret name based on template and timestamp for consistency
const kubeconfigSecretName = computed(() => {
  return `kubeconfig-${props.templateName.toLowerCase().replace(/[^a-z0-9]/g, '')}-${Date.now().toString(36)}`
})

const bindingName = computed(() => {
  // Extract binding name from authentication or use template name
  return props.bindingResponse.authentication?.oauth2CodeGrant?.sessionID || props.templateName
})

// Generate the kubectl commands
const createSecretCommand = computed(() => {
  return `kubectl create secret generic ${kubeconfigSecretName.value} --from-file=kubeconfig=./kubeconfig.yaml -n kube-bind`
})

const bindCommand = computed(() => {
  return `kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name ${kubeconfigSecretName.value} -f apiservice-export.yaml`
})

const closeModal = () => {
  emit('close')
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
  } catch (err) {
    console.error('Failed to copy command:', err)
    // Fallback for older browsers
    const textarea = document.createElement('textarea')
    textarea.value = command
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}

const downloadKubeconfig = () => {
  try {
    // Decode base64 kubeconfig
    const decodedKubeconfig = atob(props.bindingResponse.kubeconfig)
    const blob = new Blob([decodedKubeconfig], { type: 'text/yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'kubeconfig.yaml'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  } catch (error) {
    console.error('Failed to decode kubeconfig:', error)
    // Fallback: try downloading without decoding in case it's not base64
    const blob = new Blob([props.bindingResponse.kubeconfig], { type: 'text/yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'kubeconfig.yaml'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}

const downloadAPIRequests = () => {
  try {
    // Convert API requests to proper YAML format
    const apiRequestsYaml = props.bindingResponse.requests.map(req => {
      if (typeof req === 'string') {
        // If it's already a string, assume it's YAML
        return req.trim()
      } else {
        // Convert object to YAML-like format
        // Note: For proper YAML, you'd want to use a YAML library like js-yaml
        // For now, we'll format it as readable YAML-like structure
        return formatObjectAsYaml(req)
      }
    }).join('\n---\n')
    
    const blob = new Blob([apiRequestsYaml], { type: 'text/yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'apiservice-export.yaml'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  } catch (error) {
    console.error('Failed to format API requests as YAML:', error)
    // Fallback: download as JSON
    const apiRequestsJson = JSON.stringify(props.bindingResponse.requests, null, 2)
    const blob = new Blob([apiRequestsJson], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'apiservice-export.json'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}

// Helper function to format objects as proper YAML structure
const formatObjectAsYaml = (obj: any, indent: number = 0): string => {
  const spaces = ' '.repeat(indent)
  
  if (obj === null || obj === undefined) {
    return ''
  }
  
  if (typeof obj === 'string') {
    // Handle multiline strings
    if (obj.includes('\n')) {
      return `|\n${spaces}  ${obj.replace(/\n/g, `\n${spaces}  `)}`
    }
    // Quote strings that might be ambiguous in YAML
    if (obj.match(/^(true|false|null|\d+)$/i) || obj.includes(':') || obj.includes('#')) {
      return `"${obj}"`
    }
    return obj
  }
  
  if (typeof obj === 'number' || typeof obj === 'boolean') {
    return String(obj)
  }
  
  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    return obj.map(item => {
      if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
        // For object items in arrays, format them with proper indentation
        const formattedItem = formatObjectAsYaml(item, indent + 2)
        return `${spaces}- ${formattedItem.replace(/^\s*/, '')}`
      } else {
        // For simple items (strings, numbers, booleans)
        return `${spaces}- ${item}`
      }
    }).join('\n')
  }
  
  if (typeof obj === 'object') {
    return Object.entries(obj).map(([key, value]) => {
      if (value === null || value === undefined) {
        return `${spaces}${key}:`
      }
      
      if (Array.isArray(value)) {
        if (value.length === 0) {
          return `${spaces}${key}: []`
        }
        const formattedArray = formatObjectAsYaml(value, indent + 2)
        return `${spaces}${key}:\n${formattedArray}`
      }
      
      if (typeof value === 'object') {
        const formattedObject = formatObjectAsYaml(value, indent + 2)
        if (formattedObject.trim() === '') {
          return `${spaces}${key}:`
        }
        return `${spaces}${key}:\n${formattedObject}`
      }
      
      // Simple values
      const formattedValue = formatObjectAsYaml(value, 0)
      return `${spaces}${key}: ${formattedValue}`
    }).join('\n')
  }
  
  return String(obj)
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
  margin-bottom: 2rem;
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

.instructions-section {
  margin-bottom: 2rem;
}

.instructions-section h4 {
  margin-bottom: 1rem;
  color: #111827;
  font-size: 1.125rem;
  font-weight: 600;
}

.instructions-text {
  margin-bottom: 1.5rem;
  color: #6b7280;
  line-height: 1.6;
}

.command-group {
  margin-bottom: 1.5rem;
}

.download-files-section {
  margin-bottom: 1.5rem;
}

.download-files-section h5 {
  margin-bottom: 0.75rem;
  color: #374151;
  font-size: 1rem;
  font-weight: 600;
}

.download-block {
  background: #f0f9ff;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  padding: 1rem;
}

.download-text {
  margin-bottom: 1rem;
  color: #374151;
  font-size: 0.875rem;
}

.download-text code {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', 'Consolas', monospace;
  background: #e5e7eb;
  padding: 0.125rem 0.375rem;
  border-radius: 4px;
  font-size: 0.8125rem;
}

.command-group {
  margin-bottom: 1.5rem;
}

.command-group h5 {
  margin-bottom: 0.75rem;
  color: #374151;
  font-size: 1rem;
  font-weight: 600;
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

.alternative-section {
  margin-top: 2rem;
  padding-top: 1.5rem;
  border-top: 1px solid #e5e7eb;
}

.alternative-section details {
  cursor: pointer;
}

.alternative-section summary {
  font-weight: 600;
  color: #6b7280;
  padding: 0.5rem 0;
  outline: none;
}

.manual-setup {
  padding: 1rem 0;
}

.manual-setup h5 {
  margin-bottom: 1rem;
  color: #374151;
  font-weight: 600;
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
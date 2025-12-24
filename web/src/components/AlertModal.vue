<template>
  <div v-if="show" class="modal-overlay" @click="closeModal">
    <div class="modal" @click.stop>
      <div class="modal-header" :class="headerClass">
        <h3>{{ title }}</h3>
        <button @click="closeModal" class="close-btn">&times;</button>
      </div>
      
      <div class="modal-content">
        <div class="message-content">
          <div v-if="icon" class="icon-container" :class="iconClass">
            <span class="icon">{{ icon }}</span>
          </div>
          <div class="message-text">
            <pre v-if="preserveWhitespace">{{ message }}</pre>
            <p v-else>{{ message }}</p>
          </div>
        </div>
      </div>
      
      <div class="modal-footer">
        <button @click="closeModal" class="ok-btn" :class="buttonClass">
          OK
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export type AlertType = 'error' | 'warning' | 'info' | 'success'

interface Props {
  show: boolean
  title?: string
  message: string
  type?: AlertType
  preserveWhitespace?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Alert',
  type: 'error',
  preserveWhitespace: false
})

const emit = defineEmits<{
  close: []
}>()

const headerClass = computed(() => {
  switch (props.type) {
    case 'error':
      return 'header-error'
    case 'warning':
      return 'header-warning'
    case 'info':
      return 'header-info'
    case 'success':
      return 'header-success'
    default:
      return 'header-error'
  }
})

const iconClass = computed(() => {
  switch (props.type) {
    case 'error':
      return 'icon-error'
    case 'warning':
      return 'icon-warning'
    case 'info':
      return 'icon-info'
    case 'success':
      return 'icon-success'
    default:
      return 'icon-error'
  }
})

const buttonClass = computed(() => {
  switch (props.type) {
    case 'error':
      return 'btn-error'
    case 'warning':
      return 'btn-warning'
    case 'info':
      return 'btn-info'
    case 'success':
      return 'btn-success'
    default:
      return 'btn-error'
  }
})

const icon = computed(() => {
  switch (props.type) {
    case 'error':
      return '⚠️'
    case 'warning':
      return '⚠️'
    case 'info':
      return 'ℹ️'
    case 'success':
      return '✅'
    default:
      return '⚠️'
  }
})

const closeModal = () => {
  emit('close')
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
  max-width: 500px;
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
  color: white;
}

.header-error {
  background: linear-gradient(135deg, #dc2626 0%, #b91c1c 100%);
}

.header-warning {
  background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);
}

.header-info {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
}

.header-success {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
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
}

.message-content {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
}

.icon-container {
  flex-shrink: 0;
  width: 3rem;
  height: 3rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 1.5rem;
}

.icon-error {
  background: #fee2e2;
  color: #dc2626;
}

.icon-warning {
  background: #fef3c7;
  color: #f59e0b;
}

.icon-info {
  background: #dbeafe;
  color: #3b82f6;
}

.icon-success {
  background: #d1fae5;
  color: #10b981;
}

.message-text {
  flex: 1;
  min-width: 0;
}

.message-text p {
  margin: 0;
  color: #374151;
  line-height: 1.6;
  word-wrap: break-word;
}

.message-text pre {
  margin: 0;
  color: #374151;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: inherit;
}

.modal-footer {
  padding: 1.5rem 2rem;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
  display: flex;
  justify-content: flex-end;
}

.ok-btn {
  padding: 0.75rem 2rem;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s;
  color: white;
}

.btn-error {
  background: linear-gradient(135deg, #dc2626 0%, #b91c1c 100%);
}

.btn-error:hover {
  background: linear-gradient(135deg, #b91c1c 0%, #991b1b 100%);
  transform: translateY(-1px);
}

.btn-warning {
  background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);
}

.btn-warning:hover {
  background: linear-gradient(135deg, #d97706 0%, #b45309 100%);
  transform: translateY(-1px);
}

.btn-info {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
}

.btn-info:hover {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  transform: translateY(-1px);
}

.btn-success {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
}

.btn-success:hover {
  background: linear-gradient(135deg, #059669 0%, #047857 100%);
  transform: translateY(-1px);
}
</style>
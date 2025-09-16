<template>
  <div class="container">
    <div class="clusters-header">
      <h1>Cluster Management</h1>
      <p v-if="!cluster">Select a cluster to manage its resources and exports</p>
      <p v-else>Managing cluster: <span class="cluster-name">{{ cluster }}</span></p>
    </div>

    <div v-if="!cluster" class="cluster-selection">
      <div class="cluster-card default-cluster">
        <div class="cluster-header">
          <div class="cluster-icon">🏠</div>
          <div class="cluster-info">
            <h3>Default Cluster</h3>
            <p>Single cluster mode</p>
          </div>
        </div>
        <div class="cluster-actions">
          <router-link to="/resources" class="btn btn-primary">View Resources</router-link>
          <router-link to="/exports" class="btn btn-secondary">View Exports</router-link>
        </div>
      </div>

      <div class="cluster-input-section">
        <h3>Multi-Cluster Mode</h3>
        <p>Enter a specific cluster ID to manage resources for that cluster:</p>
        <div class="cluster-input-form">
          <input
            v-model="customClusterId"
            type="text"
            placeholder="Enter cluster ID"
            class="form-input"
            @keyup.enter="navigateToCluster"
          />
          <button @click="navigateToCluster" class="btn btn-primary" :disabled="!customClusterId.trim()">
            Go to Cluster
          </button>
        </div>
      </div>
    </div>

    <div v-else class="cluster-details">
      <div class="cluster-overview">
        <div class="overview-card">
          <h3>Cluster Information</h3>
          <div class="cluster-meta">
            <div class="meta-item">
              <span class="meta-label">Cluster ID:</span>
              <span class="meta-value">{{ cluster }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Mode:</span>
              <span class="meta-value">Multi-cluster</span>
            </div>
          </div>
        </div>
      </div>

      <div class="cluster-navigation">
        <div class="nav-grid">
          <div class="nav-card">
            <div class="nav-icon">🔧</div>
            <h3>Resources</h3>
            <p>Browse and bind available Kubernetes resources for this cluster</p>
            <router-link :to="`/clusters/${cluster}/resources`" class="btn btn-primary">
              View Resources
            </router-link>
          </div>

          <div class="nav-card">
            <div class="nav-icon">📦</div>
            <h3>Exports</h3>
            <p>View service exports and binding provider information</p>
            <router-link :to="`/clusters/${cluster}/exports`" class="btn btn-primary">
              View Exports
            </router-link>
          </div>

          <div class="nav-card">
            <div class="nav-icon">🔐</div>
            <h3>Authentication</h3>
            <p>Manage authentication for this cluster</p>
            <button @click="authenticateCluster" class="btn btn-primary">
              Authenticate
            </button>
          </div>
        </div>
      </div>

      <div class="cluster-actions-footer">
        <router-link to="/clusters" class="btn btn-secondary">
          ← Back to Cluster Selection
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { authService } from '../services/auth'

const router = useRouter()
const route = useRoute()

const customClusterId = ref('')

const cluster = computed(() => {
  return (route.params.cluster as string) || ''
})

const navigateToCluster = () => {
  if (customClusterId.value.trim()) {
    router.push(`/clusters/${customClusterId.value.trim()}`)
  }
}

const authenticateCluster = () => {
  authService.login(cluster.value)
}
</script>

<style scoped>
.clusters-header {
  text-align: center;
  margin-bottom: 3rem;
}

.clusters-header h1 {
  font-size: 2.5rem;
  font-weight: 700;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.clusters-header p {
  font-size: 1.1rem;
  color: #718096;
}

.cluster-name {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  color: #0366d6;
  font-weight: 600;
}

.cluster-selection {
  max-width: 600px;
  margin: 0 auto;
  space-y: 2rem;
}

.cluster-card {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
  margin-bottom: 2rem;
}

.default-cluster {
  border-left: 4px solid #28a745;
}

.cluster-header {
  display: flex;
  align-items: center;
  margin-bottom: 1.5rem;
}

.cluster-icon {
  font-size: 2.5rem;
  margin-right: 1rem;
}

.cluster-info h3 {
  font-size: 1.3rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.25rem;
}

.cluster-info p {
  color: #718096;
  font-size: 0.9rem;
}

.cluster-actions {
  display: flex;
  gap: 1rem;
}

.cluster-input-section {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
}

.cluster-input-section h3 {
  font-size: 1.3rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.cluster-input-section p {
  color: #718096;
  margin-bottom: 1rem;
}

.cluster-input-form {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.form-input {
  flex: 1;
  padding: 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 1rem;
}

.form-input:focus {
  outline: none;
  border-color: #0366d6;
  box-shadow: 0 0 0 3px rgba(3, 102, 214, 0.1);
}

.cluster-details {
  max-width: 800px;
  margin: 0 auto;
}

.cluster-overview {
  margin-bottom: 3rem;
}

.overview-card {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
}

.overview-card h3 {
  font-size: 1.3rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 1rem;
}

.cluster-meta {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
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
}

.meta-value {
  color: #2d3748;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f7fafc;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}

.nav-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

.nav-card {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  text-align: center;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
  transition: transform 0.2s, box-shadow 0.2s;
}

.nav-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 15px rgba(0, 0, 0, 0.1);
}

.nav-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.nav-card h3 {
  font-size: 1.2rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.nav-card p {
  color: #718096;
  margin-bottom: 1.5rem;
  line-height: 1.5;
}

.cluster-actions-footer {
  text-align: center;
  padding-top: 2rem;
  border-top: 1px solid #e1e4e8;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background-color: #5a6268;
}

@media (max-width: 768px) {
  .cluster-input-form {
    flex-direction: column;
  }
  
  .cluster-actions {
    flex-direction: column;
  }
  
  .nav-grid {
    grid-template-columns: 1fr;
  }
}
</style>
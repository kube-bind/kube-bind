<template>
  <div class="container">
    <div class="login-container">
      <div class="login-card">
        <div class="login-header">
          <h2>Login to Kube Bind</h2>
          <p>Authenticate via SSO to access your resources</p>
        </div>

        <div class="login-form">
          <div class="form-group">
            <label for="clusterId">Cluster ID (optional)</label>
            <input
              id="clusterId"
              v-model="clusterId"
              type="text"
              class="form-input"
              placeholder="Enter cluster ID or leave empty for default"
            />
          </div>

          <div class="form-actions">
            <button @click="handleLogin" class="btn btn-primary btn-full" :disabled="loading">
              <span v-if="loading">Authenticating...</span>
              <span v-else>Login with SSO</span>
            </button>
          </div>

          <div class="login-info">
            <p class="info-text">
              You will be redirected to your SSO provider for authentication.
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { authService } from '../services/auth'

const router = useRouter()
const clusterId = ref('')
const loading = ref(false)

const handleLogin = async () => {
  loading.value = true
  try {
    authService.login(clusterId.value)
  } catch (error) {
    console.error('Login failed:', error)
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.login-card {
  background: white;
  border-radius: 12px;
  padding: 3rem;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
  border: 1px solid #e1e4e8;
  width: 100%;
  max-width: 400px;
}

.login-header {
  text-align: center;
  margin-bottom: 2rem;
}

.login-header h2 {
  font-size: 1.8rem;
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.5rem;
}

.login-header p {
  color: #718096;
  font-size: 0.9rem;
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #4a5568;
  font-size: 0.9rem;
}

.form-input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 1rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-input:focus {
  outline: none;
  border-color: #0366d6;
  box-shadow: 0 0 0 3px rgba(3, 102, 214, 0.1);
}

.form-actions {
  margin-bottom: 1.5rem;
}

.btn-full {
  width: 100%;
  padding: 0.875rem;
  font-size: 1rem;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.login-info {
  text-align: center;
  padding-top: 1rem;
  border-top: 1px solid #e1e4e8;
}

.info-text {
  color: #6b7280;
  font-size: 0.85rem;
  line-height: 1.5;
}

@media (max-width: 480px) {
  .login-card {
    padding: 2rem 1.5rem;
    margin: 0 1rem;
  }
}
</style>
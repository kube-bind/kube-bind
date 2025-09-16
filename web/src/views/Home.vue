<template>
  <div class="container">
    <div class="hero">
      <div class="hero-content">
        <h1 class="hero-title">Welcome to Kube Bind</h1>
        <p class="hero-description">
          Bind Kubernetes resources across clusters with ease. 
          Authenticate via SSO and manage your service exports seamlessly.
        </p>
        <div class="hero-actions">
          <button v-if="!isAuthenticated" @click="login" class="btn btn-primary btn-large">
            Get Started
          </button>
          <router-link v-else to="/resources" class="btn btn-primary btn-large">
            View Resources
          </router-link>
        </div>
      </div>
    </div>

    <div class="features">
      <div class="feature-grid">
        <div class="feature-card">
          <div class="feature-icon">🔐</div>
          <h3>Secure Authentication</h3>
          <p>OAuth2/OIDC-based authentication ensures secure access to your resources.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🔗</div>
          <h3>Resource Binding</h3>
          <p>Seamlessly bind and export Kubernetes resources across different clusters.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">⚡</div>
          <h3>Fast & Reliable</h3>
          <p>Built with performance in mind for efficient resource management.</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authService } from '../services/auth'

const isAuthenticated = ref(false)

const login = () => {
  authService.login()
}

const checkAuth = async () => {
  isAuthenticated.value = await authService.checkAuthentication()
}

onMounted(() => {
  checkAuth()
})
</script>

<style scoped>
.hero {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  padding: 4rem 0;
  text-align: center;
  border-radius: 12px;
  margin-bottom: 3rem;
}

.hero-content {
  max-width: 600px;
  margin: 0 auto;
}

.hero-title {
  font-size: 3rem;
  font-weight: 700;
  margin-bottom: 1rem;
  line-height: 1.2;
}

.hero-description {
  font-size: 1.2rem;
  margin-bottom: 2rem;
  opacity: 0.9;
  line-height: 1.6;
}

.hero-actions {
  margin-top: 2rem;
}

.btn-large {
  padding: 1rem 2rem;
  font-size: 1.1rem;
  text-decoration: none;
  display: inline-block;
}

.features {
  margin-top: 4rem;
}

.feature-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 2rem;
  margin-top: 2rem;
}

.feature-card {
  background: white;
  padding: 2rem;
  border-radius: 12px;
  text-align: center;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.07);
  border: 1px solid #e1e4e8;
  transition: transform 0.2s, box-shadow 0.2s;
}

.feature-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 15px rgba(0, 0, 0, 0.1);
}

.feature-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.feature-card h3 {
  font-size: 1.3rem;
  margin-bottom: 1rem;
  color: #2c3e50;
}

.feature-card p {
  color: #718096;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .hero-title {
    font-size: 2rem;
  }
  
  .hero-description {
    font-size: 1rem;
  }
  
  .feature-grid {
    grid-template-columns: 1fr;
  }
}
</style>
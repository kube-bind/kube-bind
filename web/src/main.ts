import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Resources from './views/Resources.vue'

const routes = [
  // Default route redirects to resources
  { path: '/', redirect: '/resources' },
  
  // Main resources route
  { path: '/resources', component: Resources },
  
  // API routes that match backend endpoints - all serve the same resources view
  { path: '/api/resources', component: Resources },
  { path: '/api/clusters/:cluster/resources', component: Resources, props: true },
  
  // Web-friendly cluster-aware routes
  { path: '/clusters/:cluster/resources', component: Resources, props: true },
  
  // Catch-all route redirects to resources
  { path: '/:pathMatch(.*)*', redirect: '/resources' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

const app = createApp(App)
app.use(router)
app.mount('#app')
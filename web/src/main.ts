import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import Clusters from './views/Clusters.vue'
import Resources from './views/Resources.vue'
import Exports from './views/Exports.vue'

const routes = [
  { path: '/', component: Home },
  { path: '/login', component: Login },
  { path: '/clusters', component: Clusters },
  { path: '/resources', component: Resources },
  { path: '/exports', component: Exports },
  // API routes that match backend endpoints
  { path: '/api/resources', component: Resources },
  { path: '/api/exports', component: Exports },
  { path: '/api/clusters/:cluster/resources', component: Resources, props: true },
  { path: '/api/clusters/:cluster/exports', component: Exports, props: true },
  // Web-friendly routes
  { path: '/clusters/:cluster', component: Clusters, props: true },
  { path: '/clusters/:cluster/resources', component: Resources, props: true },
  { path: '/clusters/:cluster/exports', component: Exports, props: true }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

const app = createApp(App)
app.use(router)
app.mount('#app')
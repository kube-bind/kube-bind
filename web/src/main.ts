import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Resources from './views/Resources.vue'

const routes = [
  { path: '/', component: Resources },
  { path: '/resources', component: Resources },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

createApp(App).use(router).mount('#app')
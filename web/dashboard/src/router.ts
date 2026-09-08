import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue') },
    { path: '/node/:uuid', name: 'node-detail', component: () => import('@/views/NodeDetailView.vue') },
  ],
})

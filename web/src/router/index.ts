import { createRouter, createWebHistory } from 'vue-router'

import HomeView from '@/views/HomeView.vue'
import QueueView from '@/views/QueueView.vue'
import RepoRouteView from '@/views/RepoRouteView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/queue',
      name: 'queue',
      component: QueueView,
    },
    {
      path: '/repo/:pathMatch(.*)*',
      name: 'repo',
      component: RepoRouteView,
    },
  ],
})

export default router

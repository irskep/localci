import { createRouter, createWebHistory } from 'vue-router'

import ArtifactView from '@/views/ArtifactView.vue'
import CommitView from '@/views/CommitView.vue'
import HomeView from '@/views/HomeView.vue'
import RepoTaskHistoryView from '@/views/RepoTaskHistoryView.vue'
import RepoRouteView from '@/views/RepoRouteView.vue'
import TaskView from '@/views/TaskView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/repo',
      redirect: '/',
    },
    {
      path: '/repo/:repoPath(.*)/commit/:commit/task/:taskName/attempt/:attempt/artifact/:artifactPath(.*)',
      name: 'artifact',
      component: ArtifactView,
    },
    {
      path: '/repo/:repoPath(.*)/task/:taskName',
      name: 'repo-task-history',
      component: RepoTaskHistoryView,
    },
    {
      path: '/repo/:repoPath(.*)/commit/:commit/task/:taskName/attempt/:attempt',
      name: 'attempt',
      component: TaskView,
    },
    {
      path: '/repo/:repoPath(.*)/commit/:commit/task/:taskName',
      name: 'task',
      component: TaskView,
    },
    {
      path: '/repo/:repoPath(.*)/commit/:commit',
      name: 'commit',
      component: CommitView,
    },
    {
      path: '/repo/:pathMatch(.*)*',
      name: 'repo',
      component: RepoRouteView,
    },
  ],
})

export default router

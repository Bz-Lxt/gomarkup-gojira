import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/projects',
      name: 'projects',
      component: () => import('@/views/ProjectsView.vue'),
    },
    {
      path: '/p/:key',
      component: () => import('@/layouts/AppShell.vue'),
      children: [
        { path: '', redirect: { name: 'board' } },
        { path: 'board', name: 'board', component: () => import('@/views/BoardView.vue') },
        { path: 'backlog', name: 'backlog', component: () => import('@/views/BacklogView.vue') },
        { path: 'gantt', name: 'gantt', component: () => import('@/views/GanttView.vue') },
        { path: 'stats', name: 'stats', component: () => import('@/views/StatsView.vue') },
        { path: 'hooks', name: 'hooks', component: () => import('@/views/HooksView.vue') },
      ],
    },
    { path: '/', redirect: '/projects' },
    { path: '/:pathMatch(.*)*', redirect: '/projects' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.isAuthed && to.name === 'login') return { path: '/projects' }
    return true
  }
  if (!auth.isAuthed) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router

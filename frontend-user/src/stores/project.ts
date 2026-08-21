import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getProject, listMembers, listProjects } from '@/api/projects'
import { listSprints } from '@/api/sprints'
import type { Member, Project, Role, Sprint } from '@/api/types'
import { useAuthStore } from './auth'

export const useProjectStore = defineStore('project', () => {
  const projects = ref<Project[]>([])
  const current = ref<Project | null>(null)
  const members = ref<Member[]>([])
  const sprints = ref<Sprint[]>([])
  const loading = ref(false)

  const activeSprint = computed(() => sprints.value.find((s) => s.status === 'ACTIVE') ?? null)
  const myRole = computed<Role | null>(() => {
    const auth = useAuthStore()
    const mid = members.value.find((m) => m.user_id === auth.user?.id || m.user?.id === auth.user?.id)
    return mid?.role || current.value?.my_role || auth.role
  })

  async function loadAll() {
    loading.value = true
    try {
      projects.value = await listProjects()
    } finally {
      loading.value = false
    }
  }

  async function selectByKey(key: string) {
    if (!projects.value.length) await loadAll()
    let found = projects.value.find((p) => p.key === key || p.id === key) ?? null
    if (!found) {
      try {
        const res = await getProject(key)
        found = res.data
        if (found && !projects.value.some((p) => p.id === found!.id)) projects.value.push(found)
      } catch {
        found = null
      }
    }
    current.value = found
    if (found) {
      await Promise.all([loadMembers(found.id), loadSprints(found.id)])
    }
    return found
  }

  async function loadMembers(projectId: string) {
    members.value = await listMembers(projectId)
  }

  async function loadSprints(projectId: string) {
    sprints.value = await listSprints(projectId)
  }

  function memberName(userId?: string | null): string {
    if (!userId) return '未指派'
    const m = members.value.find((x) => x.user_id === userId || x.user?.id === userId)
    return m?.user?.display_name || m?.display_name || m?.username || m?.user?.username || '成员'
  }

  return {
    projects,
    current,
    members,
    sprints,
    loading,
    activeSprint,
    myRole,
    loadAll,
    selectByKey,
    loadMembers,
    loadSprints,
    memberName,
  }
})

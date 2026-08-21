import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Issue } from '@/api/types'

export const useUiStore = defineStore('ui', () => {
  const sidebarCollapsed = ref(false)
  const drawerOpen = ref(false)
  const drawerMode = ref<'create' | 'edit'>('edit')
  const activeIssueId = ref<string | null>(null)
  const draftType = ref<'STORY' | 'TASK' | 'BUG'>('TASK')
  const confirm = ref<{
    open: boolean
    title: string
    body: string
    resolve: ((ok: boolean) => void) | null
  }>({ open: false, title: '', body: '', resolve: null })

  function openCreate(type: 'STORY' | 'TASK' | 'BUG' = 'TASK') {
    drawerMode.value = 'create'
    draftType.value = type
    activeIssueId.value = null
    drawerOpen.value = true
  }

  function openIssue(issue: Issue | string) {
    drawerMode.value = 'edit'
    activeIssueId.value = typeof issue === 'string' ? issue : issue.id
    drawerOpen.value = true
  }

  function closeDrawer() {
    drawerOpen.value = false
    activeIssueId.value = null
  }

  function ask(title: string, body: string): Promise<boolean> {
    return new Promise((resolve) => {
      confirm.value = { open: true, title, body, resolve }
    })
  }

  function answer(ok: boolean) {
    confirm.value.resolve?.(ok)
    confirm.value = { open: false, title: '', body: '', resolve: null }
  }

  return {
    sidebarCollapsed,
    drawerOpen,
    drawerMode,
    activeIssueId,
    draftType,
    confirm,
    openCreate,
    openIssue,
    closeDrawer,
    ask,
    answer,
  }
})

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import IssueDrawer from '@/components/IssueDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import { useToastStore } from '@/stores/toast'
import { useUiStore } from '@/stores/ui'
import { ROLE_COLOR, ROLE_LABEL } from '@/utils/workflow'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const project = useProjectStore()
const ui = useUiStore()
const toast = useToastStore()

const key = computed(() => String(route.params.key || ''))

const nav = computed(() => {
  const k = key.value
  return [
    { to: `/p/${k}/board`, label: '看板', icon: 'board' },
    { to: `/p/${k}/backlog`, label: '待办', icon: 'backlog' },
    { to: `/p/${k}/gantt`, label: '甘特', icon: 'gantt' },
    { to: `/p/${k}/stats`, label: '统计', icon: 'stats' },
    { to: `/p/${k}/hooks`, label: '触发器日志', icon: 'hooks' },
  ]
})

async function resolveProject() {
  const found = await project.selectByKey(key.value)
  if (!found) toast.error('未找到该项目')
}

onMounted(resolveProject)
watch(key, resolveProject)

function logout() {
  auth.logout()
  void router.push('/login')
}

function switchProject(idOrKey: string) {
  const p = project.projects.find((x) => x.id === idOrKey || x.key === idOrKey)
  if (p) void router.push(`/p/${p.key}/board`)
}
</script>

<template>
  <div class="flex min-h-screen bg-ink text-paper">
    <aside
      class="sticky top-0 flex h-screen shrink-0 flex-col border-r border-line bg-ink transition-all duration-drawer"
      :class="ui.sidebarCollapsed ? 'w-[72px]' : 'w-[232px]'"
    >
      <div class="flex h-14 items-center gap-2 px-4">
        <span class="font-display text-lg tracking-wide">{{ ui.sidebarCollapsed ? 'GJ' : 'GoJira' }}</span>
      </div>
      <div class="px-3">
        <select
          v-if="!ui.sidebarCollapsed"
          class="field py-1.5 text-sm"
          :value="project.current?.id"
          @change="switchProject(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="p in project.projects" :key="p.id" :value="p.id">{{ p.key }} · {{ p.name }}</option>
        </select>
      </div>
      <nav class="mt-4 flex flex-1 flex-col gap-1 px-2">
        <router-link
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-paper-dim transition hover:bg-ink-3 hover:text-paper"
          active-class="!bg-ink-3 !text-paper"
        >
          <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
            <rect v-if="item.icon === 'board'" x="3" y="4" width="7" height="16" rx="1" />
            <rect v-if="item.icon === 'board'" x="14" y="4" width="7" height="10" rx="1" />
            <path v-if="item.icon === 'backlog'" d="M5 6h14M5 12h14M5 18h9" />
            <path v-if="item.icon === 'gantt'" d="M4 6h8M4 12h14M4 18h6" />
            <path v-if="item.icon === 'stats'" d="M4 18V8m6 10V4m6 14v-6" />
            <circle v-if="item.icon === 'hooks'" cx="12" cy="12" r="3" />
            <path v-if="item.icon === 'hooks'" d="M12 5v2M12 17v2M5 12h2M17 12h2" />
          </svg>
          <span v-if="!ui.sidebarCollapsed">{{ item.label }}</span>
        </router-link>
      </nav>
      <button
        type="button"
        class="m-3 rounded-lg border border-line px-2 py-1 text-xs text-paper-dim hover:text-paper"
        @click="ui.sidebarCollapsed = !ui.sidebarCollapsed"
      >
        {{ ui.sidebarCollapsed ? '»' : '收起' }}
      </button>
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex h-14 items-center justify-between border-b border-line bg-ink-2/80 px-5 backdrop-blur">
        <div class="min-w-0">
          <p class="truncate font-display text-lg">{{ project.current?.name || '载入项目…' }}</p>
          <p class="text-xs text-paper-dim">
            <span class="font-mono">{{ project.current?.key }}</span>
            <span
              v-if="project.activeSprint"
              class="ml-2 rounded-full bg-copper/20 px-2 py-0.5 text-copper"
            >{{ project.activeSprint.name }}</span>
          </p>
        </div>
        <div class="flex items-center gap-3">
          <router-link to="/projects" class="text-sm text-paper-dim hover:text-paper">全部项目</router-link>
          <div class="flex items-center gap-2 rounded-full bg-ink-3 px-3 py-1">
            <span class="h-2.5 w-2.5 rounded-full" :style="{ background: ROLE_COLOR[auth.role || 'VIEWER'] }" />
            <span class="text-sm">{{ auth.displayName }}</span>
            <span class="text-xs text-paper-dim">{{ ROLE_LABEL[auth.role || 'VIEWER'] }}</span>
          </div>
          <button type="button" class="btn-ghost rounded-lg px-3 py-1.5 text-sm" @click="logout">退出</button>
        </div>
      </header>
      <main class="page-enter min-w-0 flex-1 p-5">
        <router-view />
      </main>
    </div>
    <IssueDrawer />
  </div>
</template>

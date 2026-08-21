<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import draggable from 'vuedraggable'
import FilterBar from '@/components/FilterBar.vue'
import IssueCard from '@/components/IssueCard.vue'
import type { ColumnId, Issue } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useBoardStore } from '@/stores/board'
import { useProjectStore } from '@/stores/project'
import { useToastStore } from '@/stores/toast'
import { useUiStore } from '@/stores/ui'
import { COLUMNS } from '@/utils/workflow'

const board = useBoardStore()
const project = useProjectStore()
const auth = useAuthStore()
const ui = useUiStore()
const toast = useToastStore()
const hoverCol = ref<ColumnId | null>(null)

const ready = computed(() => Boolean(project.current?.id))

function issuesOf(id: ColumnId): Issue[] {
  return board.columns.find((c) => c.id === id)?.issues ?? []
}

function visible(issue: Issue): boolean {
  const f = board.filter
  if (f.type && issue.issue_type !== f.type) return false
  if (f.assignee && issue.assignee_id !== f.assignee) return false
  if (f.mine && issue.assignee_id !== board.currentUserId) return false
  return true
}

async function reload() {
  if (!project.current) return
  board.currentUserId = auth.user?.id || ''
  await board.load(project.current.id)
}

onMounted(reload)
watch(() => project.current?.id, reload)

function handleChange(colId: ColumnId, ev: unknown) {
  onChange(
    colId,
    ev as { added?: { element: Issue; newIndex: number }; moved?: { element: Issue; newIndex: number } },
  )
}

async function onChange(
  colId: ColumnId,
  ev: { added?: { element: Issue; newIndex: number }; moved?: { element: Issue; newIndex: number } },
) {
  const hit = ev.added || ev.moved
  if (!hit) return
  try {
    await board.moveIssue(hit.element.id, colId, hit.newIndex)
  } catch {
    toast.error('状态转换失败，已回滚')
  }
}
</script>

<template>
  <div class="flex h-full min-h-[calc(100vh-7.5rem)] flex-col">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="font-display text-3xl">看板</h2>
        <p class="text-sm text-paper-dim">拖拽推进状态 · 失败自动回滚</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <FilterBar />
        <button type="button" class="btn-primary rounded-lg px-3 py-1.5 text-sm" @click="ui.openCreate('TASK')">
          新建事项
        </button>
      </div>
    </div>

    <div v-if="!ready || board.loading" class="text-sm text-paper-dim">正在铺开列槽…</div>
    <div v-else class="grid min-h-0 flex-1 grid-cols-1 gap-3 overflow-x-auto md:grid-cols-4">
      <section
        v-for="col in COLUMNS"
        :key="col.id"
        class="flex min-w-[260px] flex-col rounded-card bg-ink-2/80 p-3 ring-1 ring-line"
        :class="{ 'column-drop-active': hoverCol === col.id }"
        @dragenter="hoverCol = col.id"
        @dragleave="hoverCol = hoverCol === col.id ? null : hoverCol"
      >
        <header class="mb-3 flex items-start justify-between gap-2">
          <div>
            <h3 class="font-display text-xl" :style="{ color: col.accent }">{{ col.label }}</h3>
            <p v-if="col.hint" class="text-xs text-paper-dim">{{ col.hint }}</p>
          </div>
          <div class="text-right font-mono text-xs text-paper-dim">
            <div>{{ board.columnMeta(col.id).count }} 卡</div>
            <div>{{ board.columnMeta(col.id).points }} pt</div>
          </div>
        </header>
        <draggable
          :list="issuesOf(col.id)"
          :animation="160"
          group="board"
          item-key="id"
          ghost-class="drag-ghost"
          class="flex min-h-[220px] flex-1 flex-col gap-2"
          @change="handleChange(col.id, $event)"
        >
          <template #item="{ element }">
            <IssueCard v-show="visible(element)" :issue="element" @click="ui.openIssue(element)" />
          </template>
          <template #footer>
            <div
              v-if="!issuesOf(col.id).filter(visible).length"
              class="grid flex-1 place-items-center rounded-lg border border-dashed border-line px-3 py-8 text-center text-sm text-paper-dim"
            >
              把卡片拖到这里
            </div>
          </template>
        </draggable>
      </section>
    </div>
  </div>
</template>

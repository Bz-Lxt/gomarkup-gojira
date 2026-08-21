<script setup lang="ts">
import type { Issue } from '@/api/types'
import { useProjectStore } from '@/stores/project'
import { initials, isOverdue, PRIORITY_COLOR, TYPE_BAR, TYPE_LABEL } from '@/utils/workflow'

defineProps<{ issue: Issue }>()

const project = useProjectStore()

function displayKey(issue: Issue) {
  return issue.key || `${project.current?.key || 'GJ'}-${issue.seq_no}`
}
</script>

<template>
  <article
    class="paper-card group relative cursor-grab overflow-hidden px-3 py-2.5 transition-transform duration-settle hover:-translate-y-px active:cursor-grabbing"
  >
    <span class="absolute inset-y-0 left-0 w-[3px]" :style="{ background: TYPE_BAR[issue.issue_type] }" />
    <div class="flex items-start justify-between gap-2 pl-1">
      <span class="font-mono text-[11px] tracking-wide text-ink/55">{{ displayKey(issue) }}</span>
      <span
        class="mt-0.5 h-2 w-2 shrink-0 rounded-full"
        :style="{ background: PRIORITY_COLOR[issue.priority] || 'var(--gold)' }"
        :title="issue.priority"
      />
    </div>
    <h4 class="mt-1.5 line-clamp-2 pl-1 text-[13.5px] font-medium leading-5 text-ink">{{ issue.title }}</h4>
    <div class="mt-2.5 flex items-center gap-2 pl-1 text-[11px] text-ink/60">
      <span class="rounded bg-ink/8 px-1.5 py-0.5">{{ TYPE_LABEL[issue.issue_type] }}</span>
      <span v-if="issue.issue_type === 'BUG' && issue.severity" class="rounded bg-rose/15 px-1.5 py-0.5 text-rose">
        {{ issue.severity }}
      </span>
      <span v-if="isOverdue(issue)" class="rounded bg-rose px-1.5 py-0.5 text-paper">逾期</span>
      <span class="ml-auto font-display text-[13px] text-ink/70">{{ issue.story_points || 0 }}</span>
      <span
        class="grid h-6 w-6 place-items-center rounded-full bg-ink-2 text-[10px] font-semibold text-paper"
        :title="issue.assignee?.display_name || project.memberName(issue.assignee_id)"
      >
        {{ initials(issue.assignee?.display_name || project.memberName(issue.assignee_id)) }}
      </span>
    </div>
  </article>
</template>

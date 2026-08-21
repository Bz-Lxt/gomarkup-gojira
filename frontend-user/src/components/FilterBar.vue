<script setup lang="ts">
import { watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useBoardStore } from '@/stores/board'
import { useProjectStore } from '@/stores/project'

const board = useBoardStore()
const project = useProjectStore()
const auth = useAuthStore()

watch(
  () => auth.user?.id,
  (id) => {
    board.currentUserId = id || ''
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <select v-model="board.filter.assignee" class="field w-auto min-w-[140px] py-1.5 text-sm">
      <option value="">全部经办人</option>
      <option v-for="m in project.members" :key="m.user_id" :value="m.user_id">
        {{ m.user?.display_name || m.display_name || m.username }}
      </option>
    </select>
    <select v-model="board.filter.type" class="field w-auto min-w-[120px] py-1.5 text-sm">
      <option value="">全部类型</option>
      <option value="STORY">故事</option>
      <option value="TASK">任务</option>
      <option value="BUG">缺陷</option>
    </select>
    <label class="inline-flex items-center gap-2 rounded-lg bg-ink-3 px-3 py-1.5 text-sm text-paper-dim">
      <input v-model="board.filter.mine" type="checkbox" class="accent-copper" />
      仅我的
    </label>
  </div>
</template>

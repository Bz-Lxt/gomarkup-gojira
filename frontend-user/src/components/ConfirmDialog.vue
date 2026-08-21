<script setup lang="ts">
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="ui.confirm.open"
      class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 px-4"
      @click.self="ui.answer(false)"
    >
      <div class="w-full max-w-md rounded-card bg-ink-2 p-6 shadow-paper ring-1 ring-line">
        <h3 class="font-display text-xl text-paper">{{ ui.confirm.title }}</h3>
        <p class="mt-3 text-sm leading-6 text-paper-dim">{{ ui.confirm.body }}</p>
        <div class="mt-6 flex justify-end gap-3">
          <button type="button" class="btn-ghost rounded-lg px-4 py-2 text-sm" @click="ui.answer(false)">取消</button>
          <button type="button" class="btn-primary rounded-lg px-4 py-2 text-sm" @click="ui.answer(true)">确认</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

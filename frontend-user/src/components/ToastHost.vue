<script setup lang="ts">
import { useToastStore } from '@/stores/toast'

const toast = useToastStore()

const tone: Record<string, string> = {
  success: 'bg-olive text-ink',
  error: 'bg-rose text-paper',
  warn: 'bg-gold text-ink',
}
</script>

<template>
  <div class="pointer-events-none fixed bottom-6 right-6 z-[80] flex w-[min(380px,calc(100vw-2rem))] flex-col gap-2">
    <div
      v-for="item in toast.items"
      :key="item.id"
      class="pointer-events-auto flex items-start gap-3 rounded-card px-4 py-3 shadow-lift"
      :class="tone[item.kind]"
    >
      <p class="flex-1 text-sm leading-5">{{ item.text }}</p>
      <button class="opacity-70 hover:opacity-100" type="button" aria-label="关闭" @click="toast.dismiss(item.id)">
        ×
      </button>
    </div>
  </div>
</template>

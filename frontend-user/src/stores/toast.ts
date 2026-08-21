import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'warn'

export interface ToastItem {
  id: number
  kind: ToastKind
  text: string
}

let seq = 1

export const useToastStore = defineStore('toast', () => {
  const items = ref<ToastItem[]>([])

  function push(kind: ToastKind, text: string, ms = 5000) {
    const id = seq++
    items.value.push({ id, kind, text })
    window.setTimeout(() => dismiss(id), ms)
  }

  function success(text: string) {
    push('success', text)
  }
  function error(text: string) {
    push('error', text)
  }
  function warn(text: string) {
    push('warn', text)
  }

  function dismiss(id: number) {
    items.value = items.value.filter((t) => t.id !== id)
  }

  return { items, push, success, error, warn, dismiss }
})

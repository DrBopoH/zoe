import { ref } from 'vue'

// Пуш-уведомления: зелёный «Снято», красный с ошибками. Процессинг синка рисует отдельная панель.
export const toasts = ref([])
let seq = 0

export const dismissToast = (id) => {
  toasts.value = toasts.value.filter(t => t.id !== id)
}

// kind: 'ok' | 'err'; ttl в мс (0 — висит, пока не закроют)
export const pushToast = ({ kind = 'ok', title, lines = [], ttl = 0 }) => {
  const id = ++seq
  toasts.value = [...toasts.value, { id, kind, title, lines }]
  if (ttl > 0) setTimeout(() => dismissToast(id), ttl)
  return id
}

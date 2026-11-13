import { ref } from 'vue'

// Пуш-уведомления справа внизу. kind: 'ok' (зелёный) | 'err' (красный) | 'warn' (янтарный, «уже идёт»). Процессинг синка рисует отдельная панель.
// Любое уведомление с ttl уходит само, у всех есть крестик; ttl 0 — висит, пока не закроют (так делать только если без решения человека не обойтись).
export const toasts = ref([])
let seq = 0
const timers = new Map()

export const dismissToast = (id) => {
  clearTimeout(timers.get(id))
  timers.delete(id)
  toasts.value = toasts.value.filter(t => t.id !== id)
}

const arm = (id, ttl) => {
  clearTimeout(timers.get(id))
  if (ttl > 0) timers.set(id, setTimeout(() => dismissToast(id), ttl))
}

// Такое же уведомление, уже висящее на экране (двойной клик по «Обновить»), не дублируется: у него только продлевается время.
export const pushToast = ({ kind = 'ok', title, lines = [], ttl = 0 }) => {
  const same = toasts.value.find(t => t.kind === kind && t.title === title && t.lines.join('\n') === lines.join('\n'))
  if (same) { arm(same.id, ttl); return same.id }
  const id = ++seq
  toasts.value = [...toasts.value, { id, kind, title, lines }]
  arm(id, ttl)
  return id
}

// Ошибка запроса: 409 («обновление уже идёт») — янтарное, коротко; остальное — красное на 10 секунд.
export const pushRequestError = (title, e) => {
  const text = String(e?.message || e)
  if (e?.status === 409) return pushToast({ kind: 'warn', title: text, ttl: 4000 })
  return pushToast({ kind: 'err', title, lines: [text], ttl: 10000 })
}

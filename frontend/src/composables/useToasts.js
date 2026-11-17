import { ref } from 'vue'
import { errText } from '../utils/errText.js'

// Пуш-уведомления справа внизу. kind: 'ok' (зелёный) | 'err' (красный) | 'warn' (янтарный, «уже идёт»). Процессинг синка рисует отдельная панель.
// Любое уведомление с ttl уходит само, у всех есть крестик; ttl 0 — висит, пока не закроют (так делать только если без решения человека не обойтись).
export const toasts = ref([])
let seq = 0
const timers = new Map()

export const dismissToast = (id) => {
  clearTimeout(timers.get(id))
  timers.delete(id)
  queue = queue.filter(x => x !== id)
  toasts.value = toasts.value.filter(t => t.id !== id)
}

// Уходят по очереди, первым — тот, кто пришёл раньше: у каждого свой таймер и своё время жизни, но истёкшие встают
// в одну очередь (по порядку появления) и покидают экран не чаще, чем раз в toastTiming.gap мс. Иначе несколько
// одновременно истёкших тостов исчезли бы разом или в случайном порядке.
export const toastTiming = { gap: 500 }
let lastLeave = 0
let queue = [] // id истёкших, по возрастанию (id растёт с появлением)
let pump = null

const drain = () => {
  if (pump || !queue.length) return
  const wait = lastLeave + toastTiming.gap - Date.now()
  const go = () => {
    pump = null
    const id = queue.shift()
    if (id !== undefined) { lastLeave = Date.now(); dismissToast(id) }
    drain()
  }
  if (wait <= 0) go()
  else pump = setTimeout(go, wait)
}

const leave = (id) => {
  timers.delete(id)
  if (!queue.includes(id)) queue = [...queue, id].sort((x, y) => x - y)
  drain()
}

const arm = (id, ttl) => {
  clearTimeout(timers.get(id))
  queue = queue.filter(x => x !== id) // повтор продлевает жизнь и снимает с очереди на уход
  if (ttl > 0) timers.set(id, setTimeout(() => leave(id), ttl))
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
  const text = errText(e)
  if (e?.status === 409) return pushToast({ kind: 'warn', title: text, ttl: 4000 })
  return pushToast({ kind: 'err', title, lines: [text], ttl: 10000 })
}

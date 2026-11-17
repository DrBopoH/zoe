import test from 'node:test'
import assert from 'node:assert/strict'
import { toasts, pushToast, dismissToast, pushRequestError, toastTiming } from '../src/composables/useToasts.js'
import { apiSend } from '../src/utils/api.js'

const clear = () => toasts.value.slice().forEach((t) => dismissToast(t.id))

test('двойной клик не плодит одинаковые уведомления', () => {
  clear()
  const a = pushToast({ kind: 'err', title: 'Не удалось обновить', lines: ['x'], ttl: 0 })
  const b = pushToast({ kind: 'err', title: 'Не удалось обновить', lines: ['x'], ttl: 0 })
  assert.equal(a, b)
  assert.equal(toasts.value.length, 1)
  pushToast({ kind: 'err', title: 'Не удалось обновить', lines: ['другое'], ttl: 0 })
  assert.equal(toasts.value.length, 2)
  clear()
})

test('уведомление с ttl уходит само, повтор продлевает время', async () => {
  clear()
  const id = pushToast({ kind: 'ok', title: 'a', ttl: 40 })
  await new Promise((r) => setTimeout(r, 25))
  assert.equal(pushToast({ kind: 'ok', title: 'a', ttl: 40 }), id) // продлено: ещё 40 мс с этого момента
  await new Promise((r) => setTimeout(r, 25))
  assert.equal(toasts.value.length, 1)
  await new Promise((r) => setTimeout(r, 40))
  assert.equal(toasts.value.length, 0)
})

test('ошибка запроса: 409 — янтарное и короткое, остальное красное с ttl', async () => {
  clear()
  globalThis.fetch = async () => ({ ok: false, status: 409, text: async () => 'обновление уже идёт' })
  const e = await apiSend('POST', '/market/sync').catch((x) => x)
  assert.equal(e.status, 409)
  pushRequestError('Не удалось обновить', e)
  assert.deepEqual(toasts.value.map((t) => [t.kind, t.title]), [['warn', 'обновление уже идёт']])
  clear()
  pushRequestError('Не удалось обновить', new Error('Failed to fetch'))
  assert.deepEqual(toasts.value.map((t) => [t.kind, t.title, t.lines[0]]), [['err', 'Не удалось обновить', 'Failed to fetch']])
  clear()
})

test('истёкшие тосты уходят по очереди: между уходами не меньше gap, свои времена жизни сохраняются', async () => {
  clear()
  toastTiming.gap = 60
  const left = []
  const watchLeave = setInterval(() => {
    for (const t of ['a', 'b', 'c']) if (!left.some((x) => x[0] === t) && !toasts.value.some((x) => x.title === t)) left.push([t, Date.now()])
  }, 2)
  pushToast({ kind: 'ok', title: 'a', ttl: 20 })
  pushToast({ kind: 'ok', title: 'b', ttl: 20 }) // истекает вместе с a
  pushToast({ kind: 'ok', title: 'c', ttl: 300 }) // своё, более долгое время жизни
  await new Promise((r) => setTimeout(r, 450))
  clearInterval(watchLeave)
  toastTiming.gap = 200
  assert.deepEqual(left.map((x) => x[0]).sort(), ['a', 'b', 'c'])
  const at = Object.fromEntries(left)
  const first = Math.min(at.a, at.b), second = Math.max(at.a, at.b)
  assert.ok(second - first >= 50, `a и b ушли с разницей ${second - first} мс`)
  assert.ok(at.c - first >= 250, 'c живёт дольше: его время не сжато очередью')
})

test('в очереди на уход раньше пришедший идёт первым', async () => {
  clear()
  toastTiming.gap = 100
  const order = []
  const seen = new Set()
  const w = setInterval(() => { for (const t of ['a', 'b', 'c']) if (!seen.has(t) && !toasts.value.some((x) => x.title === t)) { seen.add(t); order.push(t) } }, 2)
  pushToast({ kind: 'ok', title: 'a', ttl: 40 })
  pushToast({ kind: 'ok', title: 'b', ttl: 10 }) // уходит первым по сроку
  pushToast({ kind: 'ok', title: 'c', ttl: 10 }) // ждёт очереди; a истекает позже, но появился раньше c
  await new Promise((r) => setTimeout(r, 450))
  clearInterval(w)
  toastTiming.gap = 200
  assert.deepEqual(order, ['b', 'a', 'c'])
})

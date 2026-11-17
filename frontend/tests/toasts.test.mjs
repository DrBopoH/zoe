import test from 'node:test'
import assert from 'node:assert/strict'
import { toasts, pushToast, dismissToast, pushRequestError } from '../src/composables/useToasts.js'
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

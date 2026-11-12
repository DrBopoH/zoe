import test from 'node:test'
import assert from 'node:assert/strict'
import { storageGet, storageSet, loadJSON, saveJSON } from '../src/utils/storage.js'
import { apiSend, apiGetJson, apiUpload, API_BASE } from '../src/utils/api.js'

test('storage: без localStorage (бросает) приложение живёт на запасных значениях', () => {
  globalThis.localStorage = { getItem() { throw new Error('blocked') }, setItem() { throw new Error('blocked') } }
  assert.equal(storageGet('k', 'dflt'), 'dflt')
  assert.deepEqual(loadJSON('k', [1]), [1])
  const err = console.error; console.error = () => {}
  storageSet('k', 'v'); saveJSON('k', { a: 1 }) // не бросают
  console.error = err
})

test('storage: JSON туда и обратно, битый JSON → запасное', () => {
  const m = new Map()
  globalThis.localStorage = { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => m.set(k, v) }
  saveJSON('order', ['a', 'b'])
  assert.deepEqual(loadJSON('order', []), ['a', 'b'])
  m.set('bad', '{oops')
  assert.deepEqual(loadJSON('bad', 7), 7)
  assert.equal(storageGet('none', 'x'), 'x')
})

test('api: тело уходит JSON-ом, ошибка несёт текст бэкенда', async () => {
  const calls = []
  globalThis.fetch = async (url, opts) => {
    calls.push([url, opts])
    return url.endsWith('/bad')
      ? { ok: false, status: 400, text: async () => 'Invalid payload' }
      : { ok: true, status: 200, json: async () => ({ ok: 1 }), text: async () => '' }
  }
  assert.deepEqual(await apiGetJson('/accounts'), { ok: 1 })
  await apiSend('POST', '/x', { a: 1 })
  assert.equal(calls[1][0], `${API_BASE}/x`)
  assert.equal(calls[1][1].headers['Content-Type'], 'application/json')
  assert.equal(calls[1][1].body, '{"a":1}')
  await assert.rejects(() => apiSend('POST', '/bad', {}), /Invalid payload/)
  await assert.rejects(() => apiSend('GET', '/bad'), { message: 'Invalid payload' })
})

test('api: загрузка файла без ручного Content-Type', async () => {
  let seen
  globalThis.fetch = async (url, opts) => { seen = opts; return { ok: true, json: async () => ({}) } }
  const fd = { fake: 'formdata' }
  await apiUpload('/sources/inspect', fd)
  assert.equal(seen.method, 'POST')
  assert.equal(seen.body, fd)
  assert.equal(seen.headers, undefined)
})

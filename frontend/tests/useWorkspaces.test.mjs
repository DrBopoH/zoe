import test from 'node:test'
import assert from 'node:assert/strict'
import { useWorkspaces } from '../src/composables/useWorkspaces.js'

const mk = (list) => {
  const calls = []
  const get = async () => list
  const send = async (m, p, b) => { calls.push([m, p, b]); return m === 'POST' ? { id: 'n', created_at: 1, ...b } : { ok: true } }
  const raw = async (m, p) => { calls.push([m, p]) }
  return { ws: useWorkspaces(get, send, raw), calls }
}

test('update: PUT с id и правка в списке', async () => {
  const { ws, calls } = mk([{ id: 'a', name: 'Старое', color: '', image: '' }, { id: 'b', name: 'Другое', color: '', image: '' }])
  await ws.load()
  await ws.update('a', { name: 'Новое', color: '#10b981' })
  assert.deepEqual(calls[0], ['PUT', '/workspaces', { id: 'a', name: 'Новое', color: '#10b981', image: '' }])
  assert.equal(ws.list.value[0].name, 'Новое')
  assert.equal(ws.list.value[1].name, 'Другое')
})

test('remove: DELETE с id и дело уходит из списка', async () => {
  const { ws, calls } = mk([{ id: 'a/б', name: 'x' }, { id: 'b', name: 'y' }])
  await ws.load()
  await ws.remove('a/б')
  assert.equal(calls[0][0], 'DELETE')
  assert.equal(calls[0][1], '/workspaces?id=' + encodeURIComponent('a/б'))
  assert.deepEqual(ws.list.value.map((w) => w.id), ['b'])
})

test('remove: ошибка сервера оставляет дело в списке', async () => {
  const ws = useWorkspaces(async () => [{ id: 'a', name: 'x' }], async () => ({}), async () => { throw new Error('boom') })
  await ws.load()
  await assert.rejects(() => ws.remove('a'), /boom/)
  assert.equal(ws.list.value.length, 1)
})

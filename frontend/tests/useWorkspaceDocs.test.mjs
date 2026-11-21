import test from 'node:test'
import assert from 'node:assert/strict'
import { useWorkspaceDocs } from '../src/composables/useWorkspaceDocs.js'

const mk = (list = []) => {
  const calls = []
  const get = async (p) => { calls.push(['GET', p]); return list }
  const raw = async (m, p, b) => { calls.push([m, p, b]) }
  const send = async (m, p, b) => { calls.push([m, p, b]); return { id: 'wd_1', kind: b.kind, day: b.day, data: b.data } }
  return { d: useWorkspaceDocs(() => 'ws1', 'card', get, send, raw), calls }
}

test('load: документы плоские, запрос по делу и виду', async () => {
  const { d, calls } = mk([{ id: 'wd_2', kind: 'card', day: '2026-11-10', data: { substance: 'фруктоза', qty: 2 } }])
  await d.load()
  assert.deepEqual(calls[0], ['GET', '/workspaces/docs?id=ws1&kind=card'])
  assert.deepEqual(d.docs.value, [{ id: 'wd_2', day: '2026-11-10', substance: 'фруктоза', qty: 2 }])
})

test('add: поля уходят в data, служебные id/kind/day — наружу; список по дате', async () => {
  const { d, calls } = mk()
  await d.load()
  await d.add({ day: '2026-11-12', tx_id: 't1', substance: 'сорбит' })
  assert.deepEqual(calls[1], ['POST', '/workspaces/docs', { workspace_id: 'ws1', kind: 'card', day: '2026-11-12', data: { tx_id: 't1', substance: 'сорбит' } }])
  await d.add({ day: '2026-11-10', tx_id: 't2', substance: 'лимонная' })
  assert.deepEqual(d.docs.value.map((x) => x.day), ['2026-11-10', '2026-11-12'])
  assert.equal(d.docs.value[0].substance, 'лимонная')
})

test('update и remove меняют список после ответа', async () => {
  const { d, calls } = mk([{ id: 'wd_2', kind: 'card', day: '2026-11-10', data: { substance: 'фруктоза', qty: 2 } }])
  await d.load()
  await d.update({ id: 'wd_2', day: '2026-11-10', substance: 'фруктоза', qty: 3 })
  assert.deepEqual(calls[1], ['PUT', '/workspaces/docs', { workspace_id: 'ws1', id: 'wd_2', kind: 'card', day: '2026-11-10', data: { substance: 'фруктоза', qty: 3 } }])
  assert.equal(d.docs.value[0].qty, 3)
  await d.remove('wd_2')
  assert.deepEqual(calls[2], ['DELETE', '/workspaces/docs?id=ws1&doc=wd_2', undefined])
  assert.equal(d.docs.value.length, 0)
})

test('ошибка загрузки попадает в error, список не трогается', async () => {
  const d = useWorkspaceDocs(() => 'ws1', 'card', async () => { throw new Error('нет сети') })
  await d.load()
  assert.match(d.error.value, /нет сети/)
  assert.deepEqual(d.docs.value, [])
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { useWorkspaceLedger } from '../src/composables/useWorkspaceLedger.js'

const mk = (data = {}) => {
  const calls = []
  const get = async (p) => {
    if (p.startsWith('/workspaces/accounts')) return data.accounts || []
    if (p.startsWith('/workspaces/marks')) return data.marks || []
    return data.weeks || []
  }
  const raw = async (m, p, b) => { calls.push([m, p, b]) }
  return { l: useWorkspaceLedger(() => 'ws1', get, async () => ({}), raw), calls }
}

test('load: счета, пометки и недели с бэкенда', async () => {
  const { l } = mk({ accounts: ['a1'], marks: [{ tx_id: 't1', workspace_id: 'ws1', badge: 'income' }], weeks: [{ week: '2026-11-16', nominal: 2200 }] })
  await l.load()
  assert.deepEqual(l.accountIds.value, ['a1'])
  assert.equal(l.marks.value.get('t1'), 'income')
  assert.equal(l.weeks.value[0].nominal, 2200)
  assert.equal(l.linked.value.has('a1'), true)
})

test('toggleAccount: добавляет и убирает, шлёт список целиком', async () => {
  const { l, calls } = mk({ accounts: ['a1'] })
  await l.load()
  await l.toggleAccount('a2')
  assert.deepEqual(calls[0], ['PUT', '/workspaces/accounts', { workspace_id: 'ws1', accounts: ['a1', 'a2'] }])
  await l.toggleAccount('a1')
  assert.deepEqual(l.accountIds.value, ['a2'])
})

test('setMark: ставит, повтор того же бейджа снимает, другой заменяет', async () => {
  const { l, calls } = mk()
  await l.load()
  await l.setMark('t1', 'income')
  assert.equal(l.marks.value.get('t1'), 'income')
  await l.setMark('t1', 'payout')
  assert.equal(l.marks.value.get('t1'), 'payout')
  await l.setMark('t1', 'payout')
  assert.equal(l.marks.value.has('t1'), false)
  assert.deepEqual(calls[2], ['PUT', '/workspaces/marks', { workspace_id: 'ws1', tx_id: 't1', badge: '' }])
})

test('setWeek заменяет ту же неделю и держит порядок; removeWeek убирает', async () => {
  const { l, calls } = mk({ weeks: [{ week: '2026-11-16', nominal: 100 }] })
  await l.load()
  await l.setWeek('2026-11-09', 50)
  await l.setWeek('2026-11-16', 200)
  assert.deepEqual(l.weeks.value, [{ week: '2026-11-09', nominal: 50 }, { week: '2026-11-16', nominal: 200 }])
  await l.removeWeek('2026-11-09')
  assert.equal(l.weeks.value.length, 1)
  assert.deepEqual(calls[2].slice(0, 2), ['DELETE', '/workspaces/weeks?id=ws1&week=2026-11-09'])
})

test('ошибка сервера не меняет локальное состояние', async () => {
  const l = useWorkspaceLedger(() => 'ws1', async () => [], async () => ({}), async () => { throw new Error('boom') })
  await l.load()
  await assert.rejects(() => l.setMark('t1', 'income'), /boom/)
  assert.equal(l.marks.value.size, 0)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { useWorkspaceLedger } from '../src/composables/useWorkspaceLedger.js'

const mk = (data = {}) => {
  const calls = []
  const get = async (p) => {
    if (p.startsWith('/workspaces/accounts')) return data.accounts || []
    if (p.startsWith('/workspaces/marks')) return data.marks || []
    if (p.startsWith('/workspaces/params')) return data.params || []
    if (p.startsWith('/workspaces/entries')) return data.entries || []
    return data.weeks || []
  }
  const raw = async (m, p, b) => { calls.push([m, p, b]) }
  const send = async (m, p, b) => { calls.push([m, p, b]); return { id: 'we_1', ...b } }
  return { l: useWorkspaceLedger(() => 'ws1', get, send, raw), calls }
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

test('параметры: load читает, setParam и resetParam шлют на бэкенд и меняют карту', async () => {
  const { l, calls } = mk({ params: [{ key: 'tax_rate', value: 5 }] })
  await l.load()
  assert.equal(l.params.value.get('tax_rate'), 5)
  await l.setParam('esv', 1902.34)
  assert.deepEqual(calls[0], ['PUT', '/workspaces/params', { workspace_id: 'ws1', key: 'esv', value: 1902.34 }])
  assert.equal(l.params.value.get('esv'), 1902.34)
  await l.resetParam('tax_rate')
  assert.deepEqual(calls[1], ['DELETE', '/workspaces/params?id=ws1&key=tax_rate', undefined])
  assert.equal(l.params.value.has('tax_rate'), false)
})

test('наличные записи: load читает, add/update/remove шлют на бэкенд и держат порядок по дате', async () => {
  const { l, calls } = mk({ entries: [{ id: 'we_0', day: '2026-11-12', badge: 'tips', amount: 100, note: '' }] })
  await l.load()
  assert.equal(l.entries.value.length, 1)
  await l.addEntry({ day: '2026-11-10', badge: 'expense', amount: 600, note: 'парафин', tx_id: 't2' })
  assert.deepEqual(calls[0], ['POST', '/workspaces/entries', { workspace_id: 'ws1', day: '2026-11-10', badge: 'expense', amount: 600, note: 'парафин', tx_id: 't2' }])
  assert.deepEqual(l.entries.value.map((e) => e.id), ['we_1', 'we_0']) // раньше по дате — выше
  await l.updateEntry({ id: 'we_1', day: '2026-11-13', badge: 'loss', amount: 900, note: '', tx_id: 't2' })
  assert.deepEqual(calls[1], ['PUT', '/workspaces/entries', { workspace_id: 'ws1', id: 'we_1', day: '2026-11-13', badge: 'loss', amount: 900, note: '', tx_id: 't2' }])
  assert.deepEqual(l.entries.value.map((e) => e.id), ['we_0', 'we_1'])
  await l.removeEntry('we_0')
  assert.deepEqual(calls[2], ['DELETE', '/workspaces/entries?id=ws1&entry=we_0', undefined])
  assert.deepEqual(l.entries.value.map((e) => e.id), ['we_1'])
})

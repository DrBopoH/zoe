import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import WorkspaceLedger from '../../src/components/WorkspaceLedger.vue'

const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } })
const norm = (s) => s.replace(/\s/g, ' ')

const ACCOUNTS = [{ id: 'acc1', name: 'UA213223130000026007233566001', iban: 'UA213223130000026007233566001', balance: 100, currency_code: 980, type: 'white', bank_key: 'mono' }]
const TX = [
  { id: 't1', account_id: 'acc1', time: '2026-11-10T10:57:00', description: 'Rose Scarlet', amount: -2500, currency_code: 980 },
  { id: 't2', account_id: 'acc1', time: '2026-11-12T10:00:00', description: 'Glovo Ukraine', amount: 1200, currency_code: 980 }
]
let server
let calls
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  calls = []
  server = { entries: [{ id: 'we_1', day: '2026-11-10', badge: 'expense', amount: 600, note: 'парафин', tx_id: 't1' }], marks: [{ tx_id: 't1', workspace_id: 'ws1', badge: 'loss' }] }
  vi.stubGlobal('fetch', vi.fn(async (url, init = {}) => {
    const u = String(url)
    const body = init.body ? JSON.parse(init.body) : undefined
    calls.push([init.method || 'GET', u, body])
    if (u.includes('/workspaces/accounts')) return json(['acc1'])
    if (u.includes('/workspaces/marks')) return json(server.marks)
    if (u.includes('/workspaces/entries')) {
      if (init.method === 'POST') return json({ id: 'we_9', ...body })
      if (init.method === 'PUT' || init.method === 'DELETE') return json({ status: 'success' })
      return json(server.entries)
    }
    return json([])
  }))
})
afterEach(() => { vi.useRealTimers(); document.body.innerHTML = '' })

const mountLedger = async () => {
  const w = mount(WorkspaceLedger, { props: { workspaceId: 'ws1', accounts: ACCOUNTS, transactions: TX, users: [] }, attachTo: document.body })
  await flushPromises()
  return w
}
const parentRow = (w, text) => w.findAll('tbody tr').find((r) => r.text().includes(text))
const setVal = async (sel, v) => { const el = document.querySelector(sel); el.value = v; el.dispatchEvent(new Event('input')); await flushPromises() }

describe('наличные записи в истории', () => {
  it('запись рисуется под своей операцией, у операции видно «с наличными»', async () => {
    const w = await mountLedger()
    const rows = w.findAll('tbody tr').map((r) => r.attributes('data-testid') || '')
    const i = w.findAll('tbody tr').findIndex((r) => r.text().includes('Rose Scarlet'))
    expect(rows[i + 1]).toBe('cash-row')
    expect(norm(w.get('[data-testid="cash-row"]').text())).toContain('600,00')
    expect(norm(w.get('[data-testid="with-cash"]').text())).toContain('3 100,00') // −2500 − 600
    expect(w.find('[data-testid="cash-orphans"]').exists()).toBe(false)
    w.unmount()
  })

  it('в раскрытой строке «+ Наличные» открывает форму, запись уходит с привязкой к операции', async () => {
    const w = await mountLedger()
    await parentRow(w, 'Glovo Ukraine').trigger('click')
    await w.get('[data-testid="cash-add"]').trigger('click')
    document.querySelector('[data-badge="loss"]').click()
    await setVal('[data-field="amount"]', '900')
    await setVal('[data-field="note"]', 'ремонт')
    ;[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Добавить').click()
    await flushPromises()
    const post = calls.find(([m, u]) => m === 'POST' && u.endsWith('/workspaces/entries'))
    expect(post[2]).toEqual({ workspace_id: 'ws1', day: '2026-11-12', badge: 'loss', amount: 900, note: 'ремонт', tx_id: 't2' })
    expect(w.findAll('[data-testid="cash-row"]').length).toBe(2)
    expect(document.querySelector('[data-testid="cash-form"]')).toBeNull() // форма закрылась
    w.unmount()
  })

  it('правка записи на месте и удаление урной со второго нажатия', async () => {
    const w = await mountLedger()
    await parentRow(w, 'Rose Scarlet').trigger('click')
    await w.get('[data-testid="cash-edit"]').trigger('click')
    expect(document.querySelector('[data-field="amount"]').value).toBe('600')
    await setVal('[data-field="amount"]', '650')
    ;[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Сохранить').click()
    await flushPromises()
    const put = calls.find(([m, u]) => m === 'PUT' && u.endsWith('/workspaces/entries'))
    expect(put[2]).toMatchObject({ id: 'we_1', amount: 650, tx_id: 't1', day: '2026-11-10' })
    await w.get('[data-testid="cash-remove"]').trigger('click')
    expect(calls.some(([m]) => m === 'DELETE')).toBe(false)
    await w.get('[data-testid="cash-remove"]').trigger('click')
    await flushPromises()
    expect(calls.some(([m, u]) => m === 'DELETE' && u.includes('entry=we_1'))).toBe(true)
    expect(w.find('[data-testid="cash-row"]').exists()).toBe(false)
    w.unmount()
  })

  it('запись, чья операция не видна, попадает в «Наличные без операции» и считается в итогах', async () => {
    server.entries = [{ id: 'we_2', day: '2026-11-10', badge: 'loss', amount: 900, note: 'ремонт', tx_id: 'gone' }, { id: 'we_3', day: '2026-11-10', badge: 'tips', amount: 50, note: '', tx_id: '' }]
    const w = await mountLedger()
    expect(w.findAll('[data-testid="orphan-row"]').length).toBe(2)
    expect(w.find('[data-testid="cash-row"]').exists()).toBe(false)
    expect(norm(w.get('[data-testid="badge-totals"]').text())).toContain('-3 400,00') // Убыток: −2500 с карты −900 наличными
    w.unmount()
  })

  it('операции с бейджем подсвечены, без бейджа — нет', async () => {
    const w = await mountLedger()
    expect(parentRow(w, 'Rose Scarlet').classes()).toContain('row-marked') // t1 помечен «Убыток»
    expect(parentRow(w, 'Glovo Ukraine').classes()).not.toContain('row-marked')
    w.unmount()
  })
})

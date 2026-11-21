import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import GlovoPage from '../../src/views/workspaces/GlovoPage.vue'
import WorkspaceLedger from '../../src/components/WorkspaceLedger.vue'

const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } })

const ACCOUNTS = [
  { id: 'acc1', name: 'UA213223130000026007233566001', iban: 'UA213223130000026007233566001', balance: 100, currency_code: 980, type: 'white', bank_key: 'mono' }
]
const TX = [
  { id: 't1', account_id: 'acc1', time: '2026-11-16T10:00:00Z', description: 'Glovo Ukraine', amount: 1200, currency_code: 980 },
  { id: 't2', account_id: 'acc1', time: '2026-11-17T10:00:00Z', description: 'Термосумка', amount: -450.5, currency_code: 980 }
]

let calls
let server
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  calls = []
  server = { accounts: ['acc1'], marks: [{ tx_id: 't2', workspace_id: 'ws1', badge: 'expense' }], weeks: [{ week: '2026-11-16', nominal: 2200 }] }
  vi.stubGlobal('fetch', vi.fn(async (url, init = {}) => {
    const u = String(url)
    calls.push([init.method || 'GET', u, init.body ? JSON.parse(init.body) : undefined])
    if (u.includes('/workspaces/accounts')) return init.method === 'PUT' ? json({ status: 'success' }) : json(server.accounts)
    if (u.includes('/workspaces/marks')) return init.method === 'PUT' ? json({ status: 'success' }) : json(server.marks)
    if (u.includes('/workspaces/weeks')) return init.method && init.method !== 'GET' ? json({ status: 'success' }) : json(server.weeks)
    return json([])
  }))
})

afterEach(() => { vi.useRealTimers() })

const mountLedger = async () => {
  const w = mount(WorkspaceLedger, { props: { workspaceId: 'ws1', accounts: ACCOUNTS, transactions: TX, users: [] }, attachTo: document.body })
  await flushPromises()
  return w
}

describe('GlovoPage', () => {
  it('две части: Учёт по умолчанию, Смета по вкладке', async () => {
    const w = mount(GlovoPage, { props: { workspace: { id: 'ws1', name: 'Glovo' }, accounts: ACCOUNTS, transactions: TX } })
    await flushPromises()
    const tabs = w.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Учёт', 'Смета'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(w.text()).toContain('Счета дела')
    await tabs[1].trigger('click')
    await flushPromises()
    expect(w.text()).toContain('От прихода к')
    expect(w.text()).toContain('В кармане')
    expect(w.text()).not.toContain('Счета дела')
  })
})

describe('WorkspaceLedger', () => {
  it('итоги по бейджам считают помеченные операции', async () => {
    const w = await mountLedger()
    const totals = w.get('[data-testid="badge-totals"]').text()
    expect(totals).toContain('Расход')
    expect(totals).toContain('450,50')
    expect(totals).toContain('1 опер.')
    w.unmount()
  })

  it('история показывает операции привязанных счетов, помеченная — с бейджем', async () => {
    const w = await mountLedger()
    expect(w.text()).toContain('Термосумка')
    expect(w.findAll('[data-testid="row-badge"]').map((b) => b.text())).toEqual(['Расход'])
    w.unmount()
  })

  it('бейдж ставится из раскрытой строки и уходит на бэкенд', async () => {
    const w = await mountLedger()
    const row = w.findAll('tbody tr').find((r) => r.text().includes('Glovo Ukraine'))
    await row.trigger('click')
    const btn = w.findAll('button').find((b) => b.text() === 'Доход')
    await btn.trigger('click')
    await flushPromises()
    const put = calls.find(([m, u]) => m === 'PUT' && u.includes('/workspaces/marks'))
    expect(put[2]).toEqual({ workspace_id: 'ws1', tx_id: 't1', badge: 'income' })
    expect(w.findAll('[data-testid="row-badge"]').map((b) => b.text()).sort()).toEqual(['Доход', 'Расход'])
    w.unmount()
  })

  it('таймлайн: номинал вносится в карточке недели и уходит на бэкенд', async () => {
    const w = await mountLedger()
    const card = w.get('[data-week="2026-11-16"]')
    expect(card.text().replace(/\s/g, ' ')).toContain('2 200,00')
    await w.get('[data-week="2026-11-09"] .tl-add').trigger('click')
    const input = w.get('[data-week="2026-11-09"] input')
    await input.setValue('1500,5')
    await input.trigger('keydown.enter')
    await flushPromises()
    const put = calls.find(([m, u]) => m === 'PUT' && u.includes('/workspaces/weeks'))
    expect(put[2]).toEqual({ workspace_id: 'ws1', week: '2026-11-09', nominal: 1500.5 })
    expect(w.get('[data-week="2026-11-09"]').text().replace(/\s/g, ' ')).toContain('1 500,50')
    w.unmount()
  })
})

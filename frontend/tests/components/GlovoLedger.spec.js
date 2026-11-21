import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import GlovoPage from '../../src/views/workspaces/GlovoPage.vue'
import WorkspaceLedger from '../../src/components/WorkspaceLedger.vue'

const norm = (s) => s.replace(/\s/g, ' ')
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
    if (u.includes('/workspaces/marks') && u.includes('other=1')) return json(server.other || [])
    if (/\/workspaces(\?|$)/.test(u)) return json(server.list || [])
    if (u.includes('/workspaces/marks')) return init.method === 'PUT' ? json({ status: 'success' }) : json(server.marks)
    if (u.includes('/workspaces/entries')) return json(server.entries || [])
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

  it('расход от сжигания партии ХимЛаба стоит в истории по дате, только для чтения, и считается в итогах', async () => {
    server.entries = [{ id: 'we_b', day: '2026-11-11', badge: 'expense', amount: 3.84, note: 'ХимЛаб · 261115-1 · 2 шт', tx_id: '', src: 'wd_9' }]
    const w = await mountLedger()
    const rows = w.findAll('tbody tr')
    const idx = (txt) => rows.findIndex((r) => r.text().includes(txt))
    const ins = idx('ХимЛаб · 261115-1')
    expect(ins).toBeGreaterThanOrEqual(0)
    expect(rows[ins].attributes('data-testid')).toBe('insert-row')
    expect(rows[ins].text()).toContain('Расход')
    expect(w.find('[data-testid="cash-orphans"]').exists()).toBe(false) // не «наличные без операции»
    expect(norm(rows[ins].text())).toContain('−₴ 3,84')
    await rows[ins].trigger('click')
    expect(w.findAll('button').some((b) => b.text() === 'Доход')).toBe(false) // не раскрывается, бейджи не ставятся
    expect(norm(w.get('[data-testid="badge-totals"]').text())).toContain('-454,34') // 450,50 + 3,84
    w.unmount()
  })

  it('операция, уже разобранная соседним делом, тускнеет и подписана делом; действия не заблокированы', async () => {
    server.list = [{ id: 'ws1', name: 'Glovo', color: '#00aa00' }, { id: 'ws9', name: 'Семейный бюджет', color: '#3366ff' }]
    server.other = [{ tx_id: 't1', workspace_id: 'ws9', badge: 'expense' }, { tx_id: 'tx-none', workspace_id: 'ws1', badge: 'income' }]
    const w = await mountLedger()
    const chips = w.findAll('[data-testid="foreign-chip"]')
    expect(chips.length).toBe(1)
    expect(chips[0].text()).toContain('в деле «Семейный бюджет»')
    expect(chips[0].get('span').attributes('style')).toContain('#3366ff')
    const row = w.findAll('tbody tr').find((r) => r.text().includes('Glovo Ukraine'))
    expect(row.classes()).toContain('row-foreign')
    expect(w.findAll('tbody tr').filter((r) => r.classes().includes('row-foreign')).length).toBe(1)
    // не заблокирована: пометка ставится, операция переезжает в это дело и сереть перестаёт
    await row.trigger('click')
    await w.findAll('button').find((b) => b.text() === 'Доход').trigger('click')
    await flushPromises()
    expect(calls.some(([m, u]) => m === 'PUT' && u.includes('/workspaces/marks'))).toBe(true)
    expect(w.find('[data-testid="foreign-chip"]').exists()).toBe(false)
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

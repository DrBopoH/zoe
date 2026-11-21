import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import LabPage from '../../src/views/workspaces/LabPage.vue'

const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } })
const norm = (s) => s.replace(/\s/g, ' ')

const ACCOUNTS = [{ id: 'acc1', name: 'UA213223130000026007233566001', iban: 'UA213223130000026007233566001', balance: 100, currency_code: 980, type: 'white', bank_key: 'mono' }]
const TX = [
  { id: 't1', account_id: 'acc1', time: '2026-11-10T10:00:00', description: 'Хим-материал', amount: -720, currency_code: 980 },
  { id: 't2', account_id: 'acc1', time: '2026-11-11T10:00:00', description: 'Покупатель', amount: 500, currency_code: 980 }
]

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  vi.stubGlobal('fetch', vi.fn(async (url) => {
    const u = String(url)
    if (u.includes('/workspaces/accounts')) return json(['acc1'])
    if (u.includes('/workspaces/marks')) return json([{ tx_id: 't1', workspace_id: 'lab', badge: 'expense' }, { tx_id: 't2', workspace_id: 'lab', badge: 'sale' }])
    return json([])
  }))
})
afterEach(() => { vi.useRealTimers(); document.body.innerHTML = '' })

const mountLab = async () => {
  const w = mount(LabPage, { props: { workspace: { id: 'lab', name: 'ХимЛаб' }, accounts: ACCOUNTS, transactions: TX, users: [] }, attachTo: document.body })
  await flushPromises()
  return w
}

describe('LabPage', () => {
  it('четыре части: Учёт, Склад, Партии, Смета; Учёт открыт', async () => {
    const w = await mountLab()
    const tabs = w.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Учёт', 'Склад', 'Партии', 'Смета'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(w.text()).toContain('Счета дела')
    w.unmount()
  })

  it('в Учёте бейджи лаба и нет понедельного номинала Glovo', async () => {
    const w = await mountLab()
    const totals = norm(w.get('[data-testid="badge-totals"]').text())
    expect(totals).toContain('Расход')
    expect(totals).toContain('Убыток')
    expect(totals).toContain('Продажа')
    expect(totals).not.toContain('Выплата')
    expect(totals).toContain('-720,00')
    expect(totals).toContain('500,00')
    expect(w.text()).not.toContain('Номинальный доход')
    expect(w.text()).toContain('Хим-материал') // операции связанных счетов видны в истории
    expect(w.findAll('[data-testid="row-badge"]').map((b) => b.text())).toEqual(['Продажа', 'Расход'])
    w.unmount()
  })

  it('остальные части пока с подсказкой', async () => {
    const w = await mountLab()
    await w.findAll('[role="tab"]')[1].trigger('click')
    expect(w.text()).toContain('Склад появится')
    expect(w.text()).not.toContain('Счета дела')
    w.unmount()
  })
})

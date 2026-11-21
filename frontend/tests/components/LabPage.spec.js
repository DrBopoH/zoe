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

let docs
let calls
beforeEach(() => {
  docs = [{ id: 'wd_1', kind: 'card', day: '2026-11-10', data: { tx_id: 't1', substance: 'мальтодекстрин', maker: 'Nutri', net_g: 1000, price: 96, qty: 5, note: '' } }]
  calls = []
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  vi.stubGlobal('fetch', vi.fn(async (url, init = {}) => {
    const u = String(url)
    const body = init.body ? JSON.parse(init.body) : undefined
    calls.push([init.method || 'GET', u, body])
    if (u.includes('/workspaces/docs')) {
      if (init.method === 'POST') return json({ id: 'wd_9', kind: body.kind, day: body.day, data: body.data })
      if (init.method === 'PUT' || init.method === 'DELETE') return json({ status: 'success' })
      return json(docs)
    }
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

  it('карточка товара рисуется под операцией «Расход» с составом и остатком суммы', async () => {
    const w = await mountLab()
    const rows = w.findAll('tbody tr')
    const i = rows.findIndex((r) => r.text().includes('Хим-материал'))
    expect(rows[i + 1].attributes('data-testid')).toBe('card-row')
    const card = norm(rows[i + 1].text())
    expect(card).toContain('мальтодекстрин · Nutri · 1000 г')
    expect(card).toContain('5 × ₴ 96,00')
    expect(card).toContain('480,00')
    // из 720 в карточках 480, остальные 240 — доставка и прочее
    const split = norm(w.get('[data-testid="card-split"]').text())
    expect(split).toContain('В карточках ₴ 480,00')
    expect(split).toContain('из ₴ 720,00')
    expect(split).toContain('ещё ₴ 240,00')
    w.unmount()
  })

  it('«+ Товар» есть только у операции с бейджем «Расход»; форма шлёт карточку с привязкой к операции', async () => {
    const w = await mountLab()
    const rows = () => w.findAll('tbody tr')
    await rows().find((r) => r.text().includes('Покупатель')).trigger('click')
    expect(w.find('[data-testid="card-add"]').exists()).toBe(false) // «Продажа» — карточек нет
    await rows().find((r) => r.text().includes('Хим-материал')).trigger('click')
    await w.get('[data-testid="card-add"]').trigger('click')
    const set = async (f, v) => { const el = document.querySelector(`[data-field="${f}"]`); el.value = v; el.dispatchEvent(new Event('input')); await flushPromises() }
    const save = () => [...document.querySelectorAll('[data-testid="card-form"] button')].find((b) => b.textContent.includes('Добавить'))
    expect(save().disabled).toBe(true)
    await set('substance', 'фруктоза')
    await set('net_g', '1000')
    await set('price', '120')
    await set('qty', '2')
    expect(norm(document.querySelector('[data-testid="card-total"]').textContent)).toContain('240,00')
    expect(document.querySelector('#lab-substances option[value="мальтодекстрин"]')).not.toBeNull() // подсказка из уже введённых карточек
    save().click()
    await flushPromises()
    const post = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/docs'))
    expect(post[2]).toEqual({ workspace_id: 'lab', kind: 'card', day: '2026-11-10', data: { substance: 'фруктоза', maker: '', net_g: 1000, price: 120, qty: 2, note: '', tx_id: 't1' } })
    expect(w.findAll('[data-testid="card-row"]').length).toBe(2)
    w.unmount()
  })

  it('удаление карточки — в два клика', async () => {
    const w = await mountLab()
    await w.get('[data-testid="card-remove"]').trigger('click')
    expect(calls.some((c) => c[0] === 'DELETE')).toBe(false)
    await w.get('[data-testid="card-remove"]').trigger('click')
    await flushPromises()
    expect(calls.some((c) => c[0] === 'DELETE' && c[1].includes('doc=wd_1'))).toBe(true)
    expect(w.find('[data-testid="card-row"]').exists()).toBe(false)
    w.unmount()
  })
})

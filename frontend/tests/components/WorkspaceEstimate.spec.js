import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import WorkspaceEstimate from '../../src/components/WorkspaceEstimate.vue'

const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } })
const TX = [
  { id: 'p', account_id: 'x', time: '2026-11-18T10:00:00', amount: 5000, currency_code: 980 },
  { id: 'a', account_id: 'x', time: '2026-11-10T10:00:00', amount: 1000, currency_code: 980 },
  { id: 'c', account_id: 'x', time: '2026-11-17T10:00:00', amount: -200, currency_code: 980 }
]
let server
let calls
beforeEach(() => {
  calls = []
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  server = { marks: [{ tx_id: 'p', workspace_id: 'w', badge: 'payout' }, { tx_id: 'a', workspace_id: 'w', badge: 'income' }, { tx_id: 'c', workspace_id: 'w', badge: 'expense' }], weeks: [{ week: '2026-11-02', nominal: 1500 }], params: [] }
  vi.stubGlobal('fetch', vi.fn(async (url, init = {}) => {
    calls.push([init.method || 'GET', String(url), init.body ? JSON.parse(init.body) : undefined])
    const u = String(url)
    if (u.includes('/workspaces/marks')) return json(server.marks)
    if (u.includes('/workspaces/weeks')) return json(server.weeks)
    if (u.includes('/workspaces/params')) return init.method === 'PUT' || init.method === 'DELETE' ? json({ status: 'success' }) : json(server.params)
    return json([])
  }))
})
afterEach(() => { vi.useRealTimers() })

const mountIt = async () => {
  const w = mount(WorkspaceEstimate, { props: { workspaceId: 'w', transactions: TX } })
  await flushPromises()
  return w
}
const norm = (s) => s.replace(/\s/g, ' ')

describe('WorkspaceEstimate', () => {
  it('сведение: номинал, удержанное, приход, уход и «в кармане»', async () => {
    const w = await mountIt()
    const row = (k) => norm(w.get(`[data-step="${k}"]`).text())
    expect(row('nominal')).toContain('1 500,00')
    expect(row('withheld')).toContain('500,00')
    expect(row('taxable')).toContain('6 000,00')
    expect(row('out')).toContain('200,00')
    expect(row('net')).toContain('3 597,66') // 6000 − 200 − налоги 2202,34
  })

  it('график недель: по колонке на неделю от первой помеченной операции', async () => {
    const w = await mountIt()
    expect(w.findAll('[data-testid="flow-week"]').length).toBe(2)
  })

  it('без данных подсказывает, что делать', async () => {
    server.marks = []
    server.weeks = []
    const w = await mountIt()
    expect(w.text()).toContain('Смета заполнится сама')
    expect(w.find('[data-testid="flow-chart"]').exists()).toBe(false)
  })
})

describe('Налоги ФОП', () => {
  it('таблица по месяцам и «В кармане» после налогов', async () => {
    const w = await mountIt()
    const row = norm(w.get('[data-month="2026-11"]').text())
    expect(row).toContain('5 000,00') // выплаты
    expect(row).toContain('250,00') // единый налог 5%
    expect(row).toContain('50,00') // военный сбор 1%
    expect(row).toContain('1 902,34') // ЕСВ
    expect(norm(w.get('[data-step="tax"]').text())).toContain('2 202,34')
    expect(norm(w.get('[data-step="net"]').text())).toContain('3 597,66')
  })

  it('своя ставка сохраняется, значение по умолчанию сбрасывает параметр', async () => {
    const w = await mountIt()
    const input = w.get('[data-param="tax_rate"]')
    input.element.value = '3'
    await input.trigger('change')
    await flushPromises()
    expect(calls.filter(([m, u]) => m === 'PUT' && u.endsWith('/api/workspaces/params')).map((c) => c[2])).toContainEqual({ workspace_id: 'w', key: 'tax_rate', value: 3 })
    input.element.value = '5'
    await input.trigger('change')
    await flushPromises()
    expect(calls.some(([m, u]) => m === 'DELETE' && u.includes('/workspaces/params') && u.includes('key=tax_rate'))).toBe(true)
  })
})

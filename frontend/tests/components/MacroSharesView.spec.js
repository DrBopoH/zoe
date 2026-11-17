import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

const open = vi.fn()
const search = vi.fn(async () => {})
vi.mock('../../src/composables/useStocks.js', () => ({
  useStocks: () => ({ list: ref([{ ticker: 'AAPL', name: 'Apple', price: 1, market_cap: 1 }, { ticker: 'MSFT', name: 'Microsoft', price: 1, market_cap: 1 }]), detail: ref(null), loading: ref(false), error: ref(''), search, open })
}))
vi.mock('../../src/composables/useMarketRefresh.js', () => ({ useMarketRefresh: () => ({ refresh: vi.fn() }) }))
import MacroSharesView from '../../src/views/MacroSharesView.vue'

const mountView = () => mount(MacroSharesView, { props: { today: '2026-11-18' }, global: { stubs: { MacroPanel: true, MarketHeader: true, UiCard: { template: '<div><slot /></div>' }, UiEmpty: true, UiLoadError: true, UiNotice: true } } })
const flush = () => new Promise((r) => setTimeout(r))

describe('MacroSharesView: запоминание компании', () => {
  beforeEach(() => { open.mockClear(); localStorage.clear() })

  it('без сохранённой открывается первая; клик по строке запоминает тикер', async () => {
    const w = mountView()
    await flush()
    expect(open).toHaveBeenLastCalledWith('AAPL')
    await w.findAll('tbody tr')[1].trigger('click')
    expect(open).toHaveBeenLastCalledWith('MSFT')
    expect(localStorage.getItem('zoe.stocks.ticker')).toBe('MSFT')
  })

  it('после перезагрузки открывается запомненная компания', async () => {
    localStorage.setItem('zoe.stocks.ticker', 'MSFT')
    mountView()
    await flush()
    expect(open).toHaveBeenLastCalledWith('MSFT')
  })
})

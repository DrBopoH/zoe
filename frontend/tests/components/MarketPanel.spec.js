import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MarketPanel from '../../src/components/MarketPanel.vue'

const market = (feeds, started = 1000) => ({ running: true, started_at: started, feeds })

describe('MarketPanel', () => {
  afterEach(() => vi.useRealTimers())

  it('строки источников по состояниям: очередь, процент, ошибка, готово', () => {
    const w = mount(MarketPanel, {
      props: {
        market: market([
          { id: 'q', title: 'Очередь', state: 'queued' },
          { id: 'p', title: 'Курсы', state: 'running', pct: 42, note: 'окно 3/7' },
          { id: 'e', title: 'FRED', state: 'error', error: 'timeout' },
          { id: 'd', title: 'ОВДП', state: 'done', fetched: 310, unit: 'бумаг' }
        ])
      }
    })
    const t = w.text()
    expect(t).toContain('в очереди')
    expect(t).toContain('42%')
    expect(t).toContain('окно 3/7')
    expect(t).toContain('ошибка')
    expect(t).toContain('timeout')
    expect(t).toContain('Бумаг: 310')
    expect(w.find('.ui-bar--err').exists()).toBe(true)
    expect(w.find('.ui-bar--done').exists()).toBe(true)
  })

  it('без процента — бегущие полосы', () => {
    const w = mount(MarketPanel, { props: { market: market([{ id: 'r', title: 'ЕС', state: 'running' }]) } })
    expect(w.find('.ui-bar--run').exists()).toBe(true)
  })

  it('таймер идёт каждую секунду, крестик шлёт close, после размонтирования таймер снят', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(61_000)
    const w = mount(MarketPanel, { props: { market: market([{ id: 'r', title: 'X', state: 'running' }], 1) } })
    expect(w.text()).toContain('1:00')
    vi.advanceTimersByTime(5000)
    await w.vm.$nextTick()
    expect(w.text()).toContain('1:05')
    await w.find('button[aria-label="Закрыть"]').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})

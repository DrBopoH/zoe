import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SyncPanel from '../../src/components/SyncPanel.vue'

const client = (over = {}) => ({ kind: 'bank', name: 'Mono', percent: 40, finished: false, error: '', accounts: [], ...over })
const mountPanel = (progress, state = 'processing') => mount(SyncPanel, { props: { progress, state }, global: { stubs: { SyncTimeline: true } } })

describe('SyncPanel', () => {
  afterEach(() => vi.useRealTimers())

  it('без ETA и клиентов: «Считаю время…» и бегущий бар', () => {
    const w = mountPanel({ clients: [] })
    expect(w.text()).toContain('Считаю время')
    expect(w.find('.ui-bar--run').exists()).toBe(true)
  })

  it('с ETA показывает «Закончится» и осталось времени', () => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000)
    const w = mountPanel({ clients: [], eta: 1_000 + 90 })
    expect(w.text()).toContain('Закончится')
    expect(w.text()).toContain('через 2 мин')
  })

  it('состояние error: красный заголовок', () => {
    const w = mountPanel({ clients: [client({ error: 'нет доступа' })] }, 'error')
    expect(w.classes()).toContain('ui-push--err')
    expect(w.text()).toContain('Синк завершён с ошибками')
    expect(w.text()).toContain('нет доступа')
  })

  it('клиент: процент, готово — галочка вместо процента, свёрнутый — общий бар', async () => {
    const w = mountPanel({ clients: [client(), client({ kind: 'bank', name: 'Privat', percent: 100, finished: true })] })
    expect(w.text()).toContain('40%')
    expect(w.text()).not.toContain('100%')
    const first = w.findAll('button').find((b) => b.text().includes('Mono'))
    await first.trigger('click')
    expect(w.find('.ui-bar--run').exists()).toBe(true)
    await first.trigger('click')
    expect(w.find('.ui-bar--run').exists()).toBe(false)
  })

  it('крестик шлёт close', async () => {
    const w = mountPanel({ clients: [] })
    await w.find('button[aria-label="Закрыть"]').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
  })
})

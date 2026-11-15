import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MarketHeader from '../../src/components/MarketHeader.vue'

describe('MarketHeader', () => {
  it('показывает заголовок и слот', () => {
    const w = mount(MarketHeader, { props: { title: 'Курсы' }, slots: { default: '<span id="extra">поиск</span>' } })
    expect(w.find('h2').text()).toBe('Курсы')
    expect(w.find('#extra').exists()).toBe(true)
  })

  it('кнопка шлёт refresh', async () => {
    const w = mount(MarketHeader, { props: { title: 'X' } })
    expect(w.find('button').attributes('aria-label')).toBe('Обновить')
    expect(w.find('svg').classes()).not.toContain('animate-spin')
    await w.find('button').trigger('click')
    expect(w.emitted('refresh')).toHaveLength(1)
  })

  it('пока идёт опрос: кнопка занята и не шлёт refresh', async () => {
    const w = mount(MarketHeader, { props: { title: 'X', running: true } })
    const b = w.find('button')
    expect(b.attributes('aria-label')).toBe('Обновляю…')
    expect(b.attributes('aria-busy')).toBe('true')
    expect(b.find('svg').classes()).toContain('animate-spin')
    expect(b.attributes('disabled')).toBeDefined()
    await b.trigger('click')
    expect(w.emitted('refresh')).toBeUndefined()
  })

  describe('сразу после нажатия', () => {
    afterEach(() => vi.useRealTimers())

    it('кнопка занята и крутится минимум 1,5 с, даже если статус опроса ещё не пришёл', async () => {
      vi.useFakeTimers()
      const w = mount(MarketHeader, { props: { title: 'X' } })
      await w.find('button').trigger('click')
      expect(w.emitted('refresh')).toHaveLength(1)
      expect(w.find('button').attributes('aria-busy')).toBe('true')
      expect(w.find('svg').classes()).toContain('animate-spin')
      await w.find('button').trigger('click')
      expect(w.emitted('refresh')).toHaveLength(1) // второй клик за это время не уходит
      await vi.advanceTimersByTimeAsync(1600)
      expect(w.find('button').attributes('aria-busy')).toBe('false')
      expect(w.find('svg').classes()).not.toContain('animate-spin')
    })

    it('пока статус говорит «идёт», остаётся занятой и после 1,5 с', async () => {
      vi.useFakeTimers()
      const w = mount(MarketHeader, { props: { title: 'X', running: true } })
      await vi.advanceTimersByTimeAsync(1600)
      expect(w.find('button').attributes('aria-busy')).toBe('true')
    })
  })
})

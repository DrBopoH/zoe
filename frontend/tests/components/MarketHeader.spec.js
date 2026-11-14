import { describe, it, expect } from 'vitest'
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
    expect(w.find('button').text()).toBe('ОБНОВИТЬ')
    await w.find('button').trigger('click')
    expect(w.emitted('refresh')).toHaveLength(1)
  })

  it('пока идёт опрос: кнопка занята и не шлёт refresh', async () => {
    const w = mount(MarketHeader, { props: { title: 'X', running: true } })
    const b = w.find('button')
    expect(b.text()).toBe('ОБНОВЛЯЮ...')
    expect(b.attributes('disabled')).toBeDefined()
    await b.trigger('click')
    expect(w.emitted('refresh')).toBeUndefined()
  })
})

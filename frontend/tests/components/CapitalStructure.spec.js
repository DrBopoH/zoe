import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CapitalStructure from '../../src/components/CapitalStructure.vue'

const accounts = [
  { id: 'a', currency_code: 980, balance: 1000 },
  { id: 'icu', currency_code: 980, bank_key: 'icu', balance: 9000, portfolio: { kind: 'bonds', cash: 1000, invested: 8000 } },
  { id: 'bn', currency_code: 980, bank_key: 'binance', balance: 2500 }
]

describe('CapitalStructure', () => {
  it('без счетов: заглушка и нет кольца', () => {
    const w = mount(CapitalStructure)
    expect(w.text()).toContain('Нет данных по счетам')
    expect(w.find('svg').exists()).toBe(false)
  })

  it('легенда по слоям с долями, на каждый слой — дуга', () => {
    const w = mount(CapitalStructure, { props: { accounts } })
    const rows = w.findAll('ul')[0].findAll('li')
    expect(rows).toHaveLength(4) // карты, брокерский, ОВДП, крипто
    expect(rows.map((r) => r.text()).join(' ')).toMatch(/Крипто|крипто/i)
    expect(w.findAll('circle[stroke-width="14"]').length).toBe(1 + 4) // подложка + дуги слоёв
  })

  it('наведение на слой показывает его долю в центре, уход мыши — сбрасывает', async () => {
    const w = mount(CapitalStructure, { props: { accounts } })
    expect(w.text()).toContain('Капитал')
    const li = w.findAll('ul')[0].findAll('li')[0]
    await li.trigger('mouseenter')
    expect(w.text()).not.toMatch(/^Капитал$/m)
    expect(w.text()).toMatch(/\d+(,\d+)? %/)
    await li.trigger('mouseleave')
    expect(w.find('.absolute').text()).toBe('Капитал')
  })

  it('валютный счёт: без курсов не входит в капитал, с курсами — всё в долларах', () => {
    const usd = { id: 'usd', currency_code: 840, balance: 50 }
    const plain = mount(CapitalStructure, { props: { accounts: [...accounts, usd] } })
    expect(plain.text()).toContain('₴')
    expect(plain.text()).not.toContain('$')
    const rated = mount(CapitalStructure, { props: { accounts: [...accounts, usd], rates: { usdUah: 40, eurUah: 44 } } })
    expect(rated.text()).toContain('$')
  })
})

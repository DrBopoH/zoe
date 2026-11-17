import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CoverBar from '../../src/components/CoverBar.vue'

describe('CoverBar', () => {
  it('красит данные цветом банка, а не классами панели синка', () => {
    const w = mount(CoverBar, { props: { cells: '1120', from: 0, to: 1, color: '#ff0000' } })
    const runs = w.findAll('.ui-tl__run')
    expect(runs).toHaveLength(2)
    expect(runs[0].attributes('style')).toMatch(/background/)
    // happy-dom не разбирает color-mix/градиенты, поэтому у пустого отрезка проверяем только отсутствие классов панели синка
    expect(runs[1].classes()).not.toContain('ui-tl__run--empty')
  })
  it('без цвета остаются классы панели синка', () => {
    const w = mount(CoverBar, { props: { cells: '12', from: 0, to: 1 } })
    expect(w.find('.ui-tl__run--data').exists()).toBe(true)
    expect(w.find('.ui-tl__run--empty').exists()).toBe(true)
  })
})

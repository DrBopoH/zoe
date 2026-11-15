import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CurveSnapshot from '../../src/components/CurveSnapshot.vue'

const p = (d, v) => ({ d, v })
const series = {
  ust_3m: [p('2021-11-10', 0.05), p('2025-11-10', 4.1), p('2026-11-12', 4.3)],
  ust_2y: [p('2021-11-10', 0.5), p('2025-11-10', 3.9), p('2026-11-12', 4.0)],
  ust_10y: [p('2025-11-10', 4.2), p('2026-11-12', 4.6)]
}

describe('CurveSnapshot', () => {
  it('рисует три кривые и подписи сроков', () => {
    const w = mount(CurveSnapshot, { props: { series, today: '2026-11-12' } })
    expect(w.findAll('path[data-line]').map((l) => l.attributes('data-line')).sort()).toEqual(['now', 'y1', 'y5'])
    expect(w.text()).toContain('10л')
    expect(w.text()).toContain('сейчас')
    expect(w.text()).toContain('12.11.2026')
  })
  it('без данных объясняет, что нажать', () => {
    const w = mount(CurveSnapshot, { props: { series: {}, today: '2026-11-12' } })
    expect(w.find('svg').exists()).toBe(false)
    expect(w.text()).toContain('Обновить')
  })
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CoverSummary from '../../src/components/CoverSummary.vue'

const a = { id: 'a', type: 'privat', axis_from: 1000, axis_to: 2000, cells: '1120' }
const b = { id: 'b', type: 'raif', axis_from: 1500, axis_to: 2000, cells: '11' }

describe('CoverSummary', () => {
  it('рисует слои всех рисуемых счетов и подпись', () => {
    const w = mount(CoverSummary, { props: { accounts: [a, b], drawn: [a, b] } })
    expect(w.findAll('.ui-tl__run').length).toBe(3) // заливка a, штриховка a, заливка b
    expect(w.text()).toContain('все счета')
  })
  it('при выборе подписывает «выбрано N из M»', () => {
    const w = mount(CoverSummary, { props: { accounts: [a, b], drawn: [b] } })
    expect(w.text()).toContain('выбрано 1 из 2')
  })
  it('без счетов ничего не рисует', () => {
    const w = mount(CoverSummary, { props: { accounts: [], drawn: [] } })
    expect(w.find('[data-testid="cover-summary"]').exists()).toBe(false)
  })
})

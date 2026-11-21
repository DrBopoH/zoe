import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import CashEntries from '../../src/components/CashEntries.vue'

const norm = (s) => s.replace(/\s/g, ' ')
afterEach(() => { document.body.innerHTML = '' })

const ENTRIES = [
  { id: 'we_1', day: '2026-11-10', badge: 'loss', amount: 900, note: 'ремонт', tx_id: 'gone' },
  { id: 'we_2', day: '2026-11-11', badge: 'tips', amount: 50, note: '', tx_id: '' }
]

describe('CashEntries — записи без операции', () => {
  it('строки со знаком по бейджу и пометкой по умолчанию', () => {
    const w = mount(CashEntries, { props: { entries: ENTRIES } })
    const rows = w.findAll('[data-testid="orphan-row"]')
    expect(rows.length).toBe(2)
    expect(norm(rows[0].text())).toContain('−900,00')
    expect(rows[0].text()).toContain('10.11.2026')
    expect(norm(rows[1].text())).toContain('+50,00')
    expect(rows[1].text()).toContain('Наличные')
  })

  it('урна удаляет со второго нажатия', async () => {
    const w = mount(CashEntries, { props: { entries: ENTRIES } })
    const btn = w.get('[data-testid="orphan-remove"]')
    await btn.trigger('click')
    expect(w.emitted('remove')).toBeUndefined()
    await btn.trigger('click')
    expect(w.emitted('remove')[0]).toEqual(['we_1'])
  })
})

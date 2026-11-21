import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import WeekTimeline from '../../src/components/WeekTimeline.vue'

const ROWS = [
  { week: '2026-11-02', end: '2026-11-08', nominal: 2200, received: 1900, count: 2, firstDate: '2026-11-11', lagDays: 3, delta: -300, deltaPct: 13.64, status: 'paid' },
  { week: '2026-11-09', end: '2026-11-15', nominal: 1000, received: 0, count: 0, firstDate: '', lagDays: null, delta: null, deltaPct: null, status: 'pending' },
  { week: '2026-11-16', end: '2026-11-22', nominal: null, received: 0, count: 0, firstDate: '', lagDays: null, delta: null, deltaPct: null, status: 'none' }
]
// формат денег даёт неразрывный пробел: в сравнении он приводится к обычному
const norm = (s) => s.replace(/\s/g, ' ')
const mk = (rows = ROWS) => mount(WeekTimeline, { props: { rows, currentWeek: '2026-11-16' } })

describe('WeekTimeline', () => {
  it('карточка на неделю: диапазон, номинал, приход через N дней, удержано', () => {
    const w = mk()
    const paid = norm(w.get('[data-week="2026-11-02"]').text())
    expect(paid).toContain('02.11 — 08.11')
    expect(paid).toContain('2 200,00')
    expect(paid).toContain('1 900,00')
    expect(paid).toContain('через 3 дн.')
    expect(paid).toContain('Удержано 300,00 (13,64 %)')
    expect(w.get('[data-week="2026-11-09"]').text()).toContain('ждём до 22.11')
    expect(w.get('[data-week="2026-11-16"]').text()).toContain('эта неделя')
    expect(w.get('[data-week="2026-11-16"]').text()).toContain('+ Внести')
  })

  it('сводка по неделям с пришедшей выплатой', () => {
    const t = norm(mk().get('[data-testid="timeline-summary"]').text())
    expect(t).toContain('2 200,00')
    expect(t).toContain('1 900,00')
    expect(t).toContain('300,00')
    expect(t).toContain('1 нед.')
  })

  it('пустой список строк не ломает компонент', () => {
    expect(mk([]).text()).toContain('Недель пока нет')
  })

  it('внесение: Enter отдаёт save с числом (запятая ок), Esc отменяет без события', async () => {
    const w = mk()
    await w.get('[data-week="2026-11-16"] .tl-add').trigger('click')
    const input = w.get('[data-week="2026-11-16"] input')
    await input.setValue('1 234,5')
    await input.trigger('keydown.enter')
    expect(w.emitted('save')[0]).toEqual(['2026-11-16', 1234.5])
    await w.get('[data-week="2026-11-16"] .tl-add').trigger('click')
    await w.get('[data-week="2026-11-16"] input').setValue('99')
    await w.get('[data-week="2026-11-16"] input').trigger('keydown.esc')
    expect(w.emitted('save')).toHaveLength(1)
  })

  it('мусор в сумме не отправляется и подсвечивается; пусто на записанной неделе удаляет её', async () => {
    const w = mk()
    await w.get('[data-week="2026-11-16"] .tl-add').trigger('click')
    const input = w.get('[data-week="2026-11-16"] input')
    await input.setValue('abc')
    await input.trigger('keydown.enter')
    expect(w.emitted('save')).toBeUndefined()
    expect(input.classes()).toContain('!border-rose-500')
    await input.trigger('keydown.esc')
    await w.get('[data-week="2026-11-09"] .tl-value').trigger('click')
    const i2 = w.get('[data-week="2026-11-09"] input')
    await i2.setValue('')
    await i2.trigger('keydown.enter')
    expect(w.emitted('remove')[0]).toEqual(['2026-11-09'])
  })

  it('та же сумма — не событие', async () => {
    const w = mk()
    await w.get('[data-week="2026-11-09"] .tl-value').trigger('click')
    await w.get('[data-week="2026-11-09"] input').trigger('keydown.enter')
    expect(w.emitted('save')).toBeUndefined()
  })
})

describe('WeekTimeline: ещё раньше', () => {
  it('кнопка «Раньше» просит продлить ленту назад', async () => {
    const w = mk()
    await w.get('[data-testid="timeline-older"]').trigger('click')
    expect(w.emitted('older')).toHaveLength(1)
  })
})

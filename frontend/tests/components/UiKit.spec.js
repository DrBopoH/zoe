import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UiCard from '../../src/components/ui/UiCard.vue'
import UiEmpty from '../../src/components/ui/UiEmpty.vue'
import UiNotice from '../../src/components/ui/UiNotice.vue'

describe('UI-кит макро-экранов', () => {
  it('UiCard: рамка и подложка, класс родителя складывается с базовым', () => {
    const w = mount(UiCard, { attrs: { class: 'p-3 text-xs' }, slots: { default: '<i id="in">x</i>' } })
    expect(w.element.tagName).toBe('SECTION')
    expect(w.classes()).toEqual(expect.arrayContaining(['rounded-lg', 'border', 'border-slate-800', 'bg-slate-900/40', 'p-3', 'text-xs']))
    expect(w.find('#in').exists()).toBe(true)
  })

  it('UiCard as="div" рисует div', () => {
    expect(mount(UiCard, { props: { as: 'div' } }).element.tagName).toBe('DIV')
  })

  it('UiEmpty: текст из слота, единый отступ', () => {
    const w = mount(UiEmpty, { slots: { default: 'Нет данных' } })
    expect(w.text()).toBe('Нет данных')
    expect(w.classes()).toEqual(expect.arrayContaining(['px-3', 'py-3', 'text-slate-500']))
  })

  it('UiNotice: янтарная строка с текстом', () => {
    const w = mount(UiNotice, { slots: { default: 'Курса доллара пока нет' } })
    expect(w.text()).toBe('Курса доллара пока нет')
    expect(w.classes()).toContain('text-amber-300')
  })
})

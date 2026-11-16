import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UiLoadError from '../../src/components/ui/UiLoadError.vue'

describe('UiLoadError', () => {
  it('показывает текст ошибки и заголовок по умолчанию', () => {
    const w = mount(UiLoadError, { props: { message: 'Нет связи с бэкендом' } })
    expect(w.attributes('role')).toBe('alert')
    expect(w.text()).toContain('Не удалось загрузить раздел')
    expect(w.text()).toContain('Нет связи с бэкендом')
  })

  it('кнопка шлёт retry', async () => {
    const w = mount(UiLoadError, { props: { message: 'x', title: 'Свой заголовок' } })
    expect(w.text()).toContain('Свой заголовок')
    await w.find('button').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })
})

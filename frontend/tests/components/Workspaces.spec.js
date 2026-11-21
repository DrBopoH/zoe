import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import WorkspaceCard from '../../src/components/WorkspaceCard.vue'
import WorkspaceModal from '../../src/components/WorkspaceModal.vue'
import CasesView from '../../src/views/CasesView.vue'
import WorkspaceView from '../../src/views/WorkspaceView.vue'

describe('WorkspaceCard', () => {
  it('название и обложка своего цвета', () => {
    const w = mount(WorkspaceCard, { props: { workspace: { name: 'Glovo', color: '#10b981', image: '' } } })
    expect(w.text()).toContain('Glovo')
    expect(w.get('[data-testid="ws-cover"]').attributes('style')).toContain('background-color')
  })
})

describe('WorkspaceModal', () => {
  const open = () => mount(WorkspaceModal, { props: { isOpen: true }, attachTo: document.body })

  it('без названия «Создать» недоступна', async () => {
    const w = open()
    const btn = [...document.body.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Создать')
    expect(btn.disabled).toBe(true)
    w.unmount()
  })

  it('с названием отдаёт create с очищенным именем и цветом', async () => {
    const w = open()
    const input = document.body.querySelector('input[type="text"]')
    input.value = '  Glovo   курьер '
    input.dispatchEvent(new Event('input'))
    await w.vm.$nextTick()
    const btn = [...document.body.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Создать')
    btn.click()
    const ev = w.emitted('submit')
    expect(ev).toHaveLength(1)
    expect(ev[0][0]).toMatchObject({ name: 'Glovo курьер', color: '#10b981', image: '' })
    w.unmount()
  })
})

describe('CasesView', () => {
  it('есть кнопка «+» с SVG-пиктограммой, а не символом', () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } })))
    const w = mount(CasesView, { global: { stubs: { WorkspaceModal: true } } })
    const fab = w.get('button[aria-label="Новое дело"]')
    expect(fab.find('svg').exists()).toBe(true)
    expect(fab.text()).toBe('')
  })
})

describe('WorkspaceCard: карандаш и урна', () => {
  const ws = { id: 'a', name: 'Glovo', color: '#10b981', image: '' }

  it('без editable иконок нет', () => {
    expect(mount(WorkspaceCard, { props: { workspace: ws } }).findAll('button')).toHaveLength(0)
  })

  it('карандаш отдаёт edit, урна удаляет только со второго клика', async () => {
    const w = mount(WorkspaceCard, { props: { workspace: ws, editable: true } })
    await w.get('button[title="Редактировать дело"]').trigger('click')
    expect(w.emitted('edit')[0][0]).toEqual(ws)
    await w.get('button[title="Удалить дело"]').trigger('click')
    expect(w.emitted('remove')).toBeUndefined()
    expect(w.text()).toContain('Удалить?')
    await w.get('button[title="Нажми ещё раз — удалить"]').trigger('click')
    expect(w.emitted('remove')[0][0]).toEqual(ws)
  })
})

describe('WorkspaceModal: правка', () => {
  it('подставляет значения дела и отдаёт submit с ними', async () => {
    const w = mount(WorkspaceModal, { props: { isOpen: false, workspace: { id: 'a', name: 'Glovo', color: '#0ea5e9', image: 'https://e.com/a.png' } }, attachTo: document.body })
    await w.setProps({ isOpen: true })
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('Правка дела')
    const btn = [...document.body.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Сохранить')
    btn.click()
    expect(w.emitted('submit')[0][0]).toEqual({ name: 'Glovo', color: '#0ea5e9', image: 'https://e.com/a.png', page: '' })
    w.unmount()
  })
})

describe('WorkspaceCard: открытие', () => {
  const ws = { id: 'a', name: 'Glovo', color: '#10b981', image: '' }

  it('клик по карточке открывает дело, клик по иконкам — нет', async () => {
    const w = mount(WorkspaceCard, { props: { workspace: ws, editable: true } })
    await w.get('button[title="Редактировать дело"]').trigger('click')
    expect(w.emitted('open')).toBeUndefined()
    await w.get('article').trigger('click')
    expect(w.emitted('open')[0][0]).toEqual(ws)
  })

  it('в предпросмотре (без editable) карточка не открывается', async () => {
    const w = mount(WorkspaceCard, { props: { workspace: ws } })
    await w.get('article').trigger('click')
    expect(w.emitted('open')).toBeUndefined()
  })
})

describe('WorkspaceView', () => {
  it('шапка из данных дела; без страницы — заглушка; «Дела» отдаёт back', async () => {
    const w = mount(WorkspaceView, { props: { workspace: { id: 'a', name: 'Семейный бюджет', color: '#6366f1', image: '', page: '' } } })
    expect(w.get('[data-testid="ws-head"]').text()).toContain('Семейный бюджет')
    expect(w.text()).toContain('нет страницы')
    await w.get('button[aria-label="Ко всем делам"]').trigger('click')
    expect(w.emitted('back')).toHaveLength(1)
  })

  it('страница из реестра по ключу дела', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } })))
    const w = mount(WorkspaceView, { props: { workspace: { id: 'g', name: 'Glovo', color: '', image: '', page: 'glovo' } } })
    await vi.waitFor(() => expect(w.text()).toContain('Учёт'))
    expect(w.text()).toContain('Смета')
  })
})

describe('WorkspaceModal: страница', () => {
  it('выбранный ключ страницы уходит в submit', async () => {
    const w = mount(WorkspaceModal, { props: { isOpen: false }, attachTo: document.body })
    await w.setProps({ isOpen: true })
    await w.vm.$nextTick()
    const input = document.body.querySelector('input[type="text"]')
    input.value = 'Glovo'
    input.dispatchEvent(new Event('input'))
    await w.vm.$nextTick()
    const sel = document.body.querySelector('select')
    expect([...sel.options].map((o) => o.value)).toEqual(['', 'glovo', 'lab'])
    sel.value = 'glovo'
    sel.dispatchEvent(new Event('change'))
    await w.vm.$nextTick()
    ;[...document.body.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Создать').click()
    expect(w.emitted('submit')[0][0]).toMatchObject({ name: 'Glovo', page: 'glovo' })
    w.unmount()
  })
})

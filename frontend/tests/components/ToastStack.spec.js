import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ToastStack from '../../src/components/ToastStack.vue'
import { toasts, pushToast, pushRequestError } from '../../src/composables/useToasts.js'

const mountStack = (props = {}) => mount(ToastStack, { props })

describe('ToastStack', () => {
  beforeEach(() => { toasts.value = []; vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('рисует уведомление с заголовком, строками и цветом по kind', async () => {
    const w = mountStack()
    pushToast({ kind: 'err', title: 'Сбой', lines: ['a', 'b'] })
    await w.vm.$nextTick()
    const t = w.find('.ui-push')
    expect(t.classes()).toContain('ui-push--err')
    expect(t.text()).toContain('Сбой')
    expect(t.text()).toContain('a')
    expect(t.text()).toContain('b')
  })

  it('крестик убирает уведомление', async () => {
    const w = mountStack()
    pushToast({ title: 'Снято' })
    await w.vm.$nextTick()
    await w.find('button[aria-label="Закрыть"]').trigger('click')
    expect(toasts.value).toHaveLength(0)
    expect(w.find('.ui-push').exists()).toBe(false)
  })

  it('уходит по ttl сам', async () => {
    const w = mountStack()
    pushToast({ title: 'Снято', ttl: 3000 })
    await w.vm.$nextTick()
    vi.advanceTimersByTime(2999)
    expect(toasts.value).toHaveLength(1)
    vi.advanceTimersByTime(1)
    await w.vm.$nextTick()
    expect(toasts.value).toHaveLength(0)
  })

  it('двойной клик не дублирует, а продлевает время', async () => {
    pushToast({ title: 'Обновляю', ttl: 3000 })
    vi.advanceTimersByTime(2000)
    pushToast({ title: 'Обновляю', ttl: 3000 })
    expect(toasts.value).toHaveLength(1)
    vi.advanceTimersByTime(2000)
    expect(toasts.value).toHaveLength(1)
    vi.advanceTimersByTime(1000)
    expect(toasts.value).toHaveLength(0)
  })

  it('409 — янтарное и короткое, остальное — красное с текстом ошибки', () => {
    pushRequestError('Не вышло', Object.assign(new Error('уже идёт'), { status: 409 }))
    pushRequestError('Не вышло', new Error('boom'))
    expect(toasts.value[0]).toMatchObject({ kind: 'warn', title: 'уже идёт', lines: [] })
    expect(toasts.value[1]).toMatchObject({ kind: 'err', title: 'Не вышло', lines: ['boom'] })
  })

  it('потеря связи: таймер, крестик прячет именно этот разрыв', async () => {
    const t0 = Date.now()
    const w = mountStack({ lostSince: t0 })
    expect(w.text()).toContain('Нет ответа от бэкенда · 0:00')
    vi.advanceTimersByTime(65_000)
    await w.vm.$nextTick()
    expect(w.text()).toContain('1:05')
    await w.find('button[aria-label="Скрыть"]').trigger('click')
    expect(w.text()).not.toContain('Нет ответа')
    await w.setProps({ lostSince: t0 + 100_000 }) // новый разрыв показывается снова
    expect(w.text()).toContain('Нет ответа')
  })
})

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import LabPage from '../../src/views/workspaces/LabPage.vue'

const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } })
const norm = (s) => s.replace(/\s/g, ' ')

const ACCOUNTS = [{ id: 'acc1', name: 'UA213223130000026007233566001', iban: 'UA213223130000026007233566001', balance: 100, currency_code: 980, type: 'white', bank_key: 'mono' }]
const TX = [
  { id: 't1', account_id: 'acc1', time: '2026-11-10T10:00:00', description: 'Хим-материал', amount: -720, currency_code: 980 },
  { id: 't2', account_id: 'acc1', time: '2026-11-11T10:00:00', description: 'Покупатель', amount: 500, currency_code: 980 }
]

let docs
let calls
let failEntries
beforeEach(() => {
  failEntries = false
  docs = [{ id: 'wd_1', kind: 'card', day: '2026-11-10', data: { tx_id: 't1', substance: 'мальтодекстрин', maker: 'Nutri', net_g: 1000, price: 96, qty: 5, note: '' } }]
  calls = []
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-21T12:00:00'))
  vi.stubGlobal('fetch', vi.fn(async (url, init = {}) => {
    const u = String(url)
    const body = init.body ? JSON.parse(init.body) : undefined
    calls.push([init.method || 'GET', u, body])
    if (u.includes('/workspaces/docs')) {
      if (init.method === 'POST') return json({ id: 'wd_9', kind: body.kind, day: body.day, data: body.data })
      if (init.method === 'PUT' || init.method === 'DELETE') return json({ status: 'success' })
      const kind = new URL(u, 'http://x').searchParams.get('kind')
      return json(docs.filter((d) => !kind || d.kind === kind))
    }
    if (/\/workspaces(\?|$)/.test(u)) return json([{ id: 'lab', name: 'ХимЛаб', color: '' }, { id: 'glovo', name: 'Glovo', color: '#00aa00' }])
    if (u.includes('/workspaces/entries') && init.method === 'POST') return failEntries ? new Response('{"error":"boom"}', { status: 500, headers: { 'Content-Type': 'application/json' } }) : json({ id: 'we_1', ...body })
    if (u.includes('/workspaces/accounts')) return json(['acc1'])
    if (u.includes('/workspaces/marks')) return json([{ tx_id: 't1', workspace_id: 'lab', badge: 'expense' }, { tx_id: 't2', workspace_id: 'lab', badge: 'sale' }])
    return json([])
  }))
})
afterEach(() => { vi.useRealTimers(); document.body.innerHTML = '' })

const mountLab = async () => {
  const w = mount(LabPage, { props: { workspace: { id: 'lab', name: 'ХимЛаб' }, accounts: ACCOUNTS, transactions: TX, users: [] }, attachTo: document.body })
  await flushPromises()
  return w
}

describe('LabPage', () => {
  it('четыре части: Учёт, Склад, Партии, Смета; Учёт открыт', async () => {
    const w = await mountLab()
    const tabs = w.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Учёт', 'Склад', 'Партии', 'Смета'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(w.text()).toContain('Счета дела')
    w.unmount()
  })

  it('в Учёте бейджи лаба и нет понедельного номинала Glovo', async () => {
    const w = await mountLab()
    const totals = norm(w.get('[data-testid="badge-totals"]').text())
    expect(totals).toContain('Расход')
    expect(totals).toContain('Убыток')
    expect(totals).toContain('Продажа')
    expect(totals).not.toContain('Выплата')
    expect(totals).toContain('-720,00')
    expect(totals).toContain('500,00')
    expect(w.text()).not.toContain('Номинальный доход')
    expect(w.text()).toContain('Хим-материал') // операции связанных счетов видны в истории
    expect(w.findAll('[data-testid="row-badge"]').map((b) => b.text())).toEqual(['Продажа', 'Расход'])
    w.unmount()
  })

  it('остальные части пока с подсказкой', async () => {
    const w = await mountLab()
    await w.findAll('[role="tab"]')[3].trigger('click')
    expect(w.text()).toContain('Смета лаба появится')
    expect(w.text()).not.toContain('Счета дела')
    w.unmount()
  })

  it('карточка товара рисуется под операцией «Расход» с составом и остатком суммы', async () => {
    const w = await mountLab()
    const rows = w.findAll('tbody tr')
    const i = rows.findIndex((r) => r.text().includes('Хим-материал'))
    expect(rows[i + 1].attributes('data-testid')).toBe('card-row')
    const card = norm(rows[i + 1].text())
    expect(card).toContain('мальтодекстрин · Nutri · 1000 г')
    expect(card).toContain('5 × ₴ 96,00')
    expect(card).toContain('480,00')
    // из 720 в карточках 480, остальные 240 — доставка и прочее
    const split = norm(w.get('[data-testid="card-split"]').text())
    expect(split).toContain('В карточках ₴ 480,00')
    expect(split).toContain('из ₴ 720,00')
    expect(split).toContain('ещё ₴ 240,00')
    w.unmount()
  })

  it('«+ Товар» есть только у операции с бейджем «Расход»; форма шлёт карточку с привязкой к операции', async () => {
    const w = await mountLab()
    const rows = () => w.findAll('tbody tr')
    await rows().find((r) => r.text().includes('Покупатель')).trigger('click')
    expect(w.find('[data-testid="card-add"]').exists()).toBe(false) // «Продажа» — карточек нет
    await rows().find((r) => r.text().includes('Хим-материал')).trigger('click')
    await w.get('[data-testid="card-add"]').trigger('click')
    const set = async (f, v) => { const el = document.querySelector(`[data-field="${f}"]`); el.value = v; el.dispatchEvent(new Event('input')); await flushPromises() }
    const save = () => [...document.querySelectorAll('[data-testid="card-form"] button')].find((b) => b.textContent.includes('Добавить'))
    expect(save().disabled).toBe(true)
    await set('substance', 'фруктоза')
    await set('net_g', '1000')
    await set('price', '120')
    await set('qty', '2')
    expect(norm(document.querySelector('[data-testid="card-total"]').textContent)).toContain('240,00')
    expect(document.querySelector('#lab-substances option[value="мальтодекстрин"]')).not.toBeNull() // подсказка из уже введённых карточек
    save().click()
    await flushPromises()
    const post = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/docs'))
    expect(post[2]).toEqual({ workspace_id: 'lab', kind: 'card', day: '2026-11-10', data: { substance: 'фруктоза', maker: '', net_g: 1000, price: 120, qty: 2, note: '', tx_id: 't1' } })
    expect(w.findAll('[data-testid="card-row"]').length).toBe(2)
    w.unmount()
  })

  it('удаление карточки — в два клика', async () => {
    const w = await mountLab()
    await w.get('[data-testid="card-remove"]').trigger('click')
    expect(calls.some((c) => c[0] === 'DELETE')).toBe(false)
    await w.get('[data-testid="card-remove"]').trigger('click')
    await flushPromises()
    expect(calls.some((c) => c[0] === 'DELETE' && c[1].includes('doc=wd_1'))).toBe(true)
    expect(w.find('[data-testid="card-row"]').exists()).toBe(false)
    w.unmount()
  })

  it('Склад: вещества с карточками, запас от номинала, цены карточек раздельно', async () => {
    docs.push(
      { id: 'wd_2', kind: 'card', day: '2026-11-12', data: { tx_id: 't1', substance: 'Мальтодекстрин', maker: 'Другой', net_g: 500, price: 60, qty: 2, note: '' } },
      { id: 'wd_3', kind: 'card', day: '2026-11-12', data: { tx_id: 't1', substance: 'фруктоза', maker: '', net_g: 1000, price: 120, qty: 2, note: '' } }
    )
    const w = await mountLab()
    await w.findAll('[role="tab"]')[1].trigger('click')
    await flushPromises()
    const groups = w.findAll('[data-testid="stock-group"]')
    expect(groups.map((g) => g.find('header div').text())).toEqual(['мальтодекстрин', 'фруктоза']) // «Мальтодекстрин» с большой буквы — то же вещество, группы по алфавиту
    const malt = norm(groups[0].text())
    expect(malt).toContain('6000 г из 6000 г') // 5×1000 + 2×500
    expect(malt).toContain('600,00') // 480 + 2×60: цены карточек не усреднены
    expect(malt).toContain('0,096 ₴/г')
    expect(malt).toContain('0,12 ₴/г')
    expect(groups[0].findAll('[data-testid="stock-card"]').length).toBe(2)
    expect(w.findAll('[data-testid="stock-bar"]')[0].attributes('style')).toContain('100%')
    const totals = norm(w.get('[data-testid="stock-totals"]').text())
    expect(totals).toContain('Веществ2')
    expect(totals).toContain('3 карточ.')
    expect(totals).toContain('840,00') // 600 + 240
    w.unmount()
  })

  it('Склад без карточек подсказывает, где их вносить', async () => {
    docs.length = 0
    const w = await mountLab()
    await w.findAll('[role="tab"]')[1].trigger('click')
    await flushPromises()
    expect(w.text()).toContain('+ Товар')
    w.unmount()
  })

  const setBy = async (el, v) => { el.value = v; el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input')); await flushPromises() }

  it('Партии: форма считает себестоимость по цене грамма карточки и сохраняет партию с номером ГГММДД-N', async () => {
    const w = await mountLab()
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Партий пока нет')
    await w.get('[data-testid="batch-new"]').trigger('click')
    expect(document.querySelector('[data-field="code"]').value).toBe('261121-1')
    await setBy(document.querySelector('[data-field="pieces"]'), '10')
    await setBy(document.querySelector('[data-field="card"]'), 'wd_1')
    await setBy(document.querySelector('[data-field="grams"]'), '200') // 200 г × 0,096
    expect(norm(document.querySelector('[data-testid="batch-cost"]').textContent)).toContain('19,20')
    expect(document.querySelector('[data-field="per"]').value).toBe('20') // 20 г на штуку
    expect(norm(document.querySelector('[data-testid="batch-cost"]').textContent)).toContain('1,92') // штука
    const save = [...document.querySelectorAll('[data-testid="batch-form"] button')].find((b) => b.textContent.includes('Создать'))
    save.click()
    await flushPromises()
    const post = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/docs'))
    expect(post[2].kind).toBe('batch')
    expect(post[2].data).toEqual({ code: '261121-1', pieces: 10, note: '', lines: [{ card_id: 'wd_1', grams: 200 }] })
    expect(norm(w.get('[data-testid="batch"]').text())).toContain('261121-1')
    expect(norm(w.get('[data-testid="batch-piece"]').text())).toContain('1,92')
    w.unmount()
  })

  it('Партии: нельзя взять больше остатка карточки, расход виден на Складе', async () => {
    docs.push({ id: 'wd_b', kind: 'batch', day: '2026-11-15', data: { code: '261115-1', pieces: 0, note: '', lines: [{ card_id: 'wd_1', grams: 4900 }] } })
    const w = await mountLab()
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    await w.get('[data-testid="batch-new"]').trigger('click')
    expect(document.querySelector('[data-field="code"]').value).toBe('261121-1')
    await setBy(document.querySelector('[data-field="card"]'), 'wd_1')
    await setBy(document.querySelector('[data-field="grams"]'), '200') // осталось 5000 − 4900 = 100 г
    expect(document.querySelector('[data-testid="batch-over"]')).not.toBeNull()
    const save = [...document.querySelectorAll('[data-testid="batch-form"] button')].find((b) => b.textContent.includes('Создать'))
    expect(save.disabled).toBe(true)
    await setBy(document.querySelector('[data-field="grams"]'), '100')
    expect(save.disabled).toBe(false)
    await w.findAll('[role="tab"]')[1].trigger('click')
    await flushPromises()
    expect(norm(w.get('[data-testid="stock-group"]').text())).toContain('100 г из 5000 г')
    w.unmount()
  })

  it('Партии: состав раскрывается, удаление в два клика', async () => {
    docs.push({ id: 'wd_b', kind: 'batch', day: '2026-11-15', data: { code: '261115-1', pieces: 10, note: 'тест', lines: [{ card_id: 'wd_1', grams: 200 }, { card_id: 'gone', grams: 5 }] } })
    const w = await mountLab()
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    await w.get('[data-testid="batch"] .group').trigger('click')
    const lines = norm(w.get('[data-testid="batch-lines"]').text())
    expect(lines).toContain('мальтодекстрин · Nutri · 1000 г')
    expect(lines).toContain('20 г/шт')
    expect(lines).toContain('карточка удалена')
    await w.get('[data-testid="batch-remove"]').trigger('click')
    expect(calls.some((c) => c[0] === 'DELETE')).toBe(false)
    await w.get('[data-testid="batch-remove"]').trigger('click')
    await flushPromises()
    expect(calls.some((c) => c[0] === 'DELETE' && c[1].includes('doc=wd_b'))).toBe(true)
    expect(w.find('[data-testid="batch"]').exists()).toBe(false)
    w.unmount()
  })

  it('Партии: дробные граммы на штуку не теряются (16,667 г/шт — с точкой в поле)', async () => {
    const w = await mountLab()
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    await w.get('[data-testid="batch-new"]').trigger('click')
    await setBy(document.querySelector('[data-field="pieces"]'), '6')
    await setBy(document.querySelector('[data-field="card"]'), 'wd_1')
    await setBy(document.querySelector('[data-field="grams"]'), '100.125')
    expect(document.querySelector('[data-field="per"]').value).toBe('16.688')
    await setBy(document.querySelector('[data-field="per"]'), '0.125')
    expect(document.querySelector('[data-field="grams"]').value).toBe('0.75')
    w.unmount()
  })

  const BATCH = { id: 'wd_b', kind: 'batch', day: '2026-11-15', data: { code: '261115-1', pieces: 10, note: '', lines: [{ card_id: 'wd_1', grams: 200 }] } } // 19,20 ₴, штука 1,92 ₴
  const openBatch = async (w) => {
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    await w.get('[data-testid="batch"] .group').trigger('click')
  }

  it('Продажа партии: штуки и цена, выручка и наценка, остаток штук', async () => {
    docs.push(BATCH)
    const w = await mountLab()
    await openBatch(w)
    expect(w.get('[data-testid="batch-left"]').text()).toBe('10')
    await w.get('[data-testid="move-sale"]').trigger('click')
    await setBy(document.querySelector('[data-testid="move-form"] [data-field="pieces"]'), '4')
    await setBy(document.querySelector('[data-testid="move-form"] [data-field="price"]'), '15')
    expect(norm(document.querySelector('[data-testid="move-amount"]').textContent)).toContain('Выручка 60,00') // 4 × 15
    expect(norm(document.querySelector('[data-testid="move-amount"]').textContent)).toContain('наценка 52,32') // 60 − 4×1,92
    ;[...document.querySelectorAll('[data-testid="move-form"] button')].find((b) => b.textContent.includes('Продать')).click()
    await flushPromises()
    const post = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/docs') && c[2].kind === 'move')
    expect(post[2].data).toEqual({ type: 'sale', batch_id: 'wd_b', pieces: 4, note: '', price: 15 })
    expect(calls.some((c) => c[1].includes('/workspaces/entries') && c[0] === 'POST')).toBe(false) // продажа расхода не пишет
    expect(w.get('[data-testid="batch-left"]').text()).toBe('6')
    expect(norm(w.get('[data-testid="move-row"]').text())).toContain('4 шт × 15,00')
    w.unmount()
  })

  it('Сжигание: движение и расход по себестоимости в выбранном деле с источником; нельзя больше остатка', async () => {
    docs.push(BATCH)
    const w = await mountLab()
    await openBatch(w)
    await w.get('[data-testid="move-burn"]').trigger('click')
    const opts = [...document.querySelectorAll('[data-testid="move-form"] [data-field="target"] option')].map((o) => o.textContent)
    expect(opts).toEqual(['Glovo']) // само дело в списке не предлагается
    await setBy(document.querySelector('[data-testid="move-form"] [data-field="pieces"]'), '11')
    expect(document.querySelector('[data-testid="move-over"]')).not.toBeNull()
    await setBy(document.querySelector('[data-testid="move-form"] [data-field="pieces"]'), '2')
    expect(norm(document.querySelector('[data-testid="move-amount"]').textContent)).toContain('3,84') // 2 × 1,92
    ;[...document.querySelectorAll('[data-testid="move-form"] button')].find((b) => b.textContent.includes('Сжечь')).click()
    await flushPromises()
    const doc = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/docs') && c[2].kind === 'move')
    expect(doc[2].data).toMatchObject({ type: 'burn', pieces: 2, target_ws: 'glovo', target_name: 'Glovo' })
    const entry = calls.find((c) => c[0] === 'POST' && c[1].includes('/workspaces/entries'))
    expect(entry[2]).toEqual({ workspace_id: 'glovo', day: '2026-11-21', badge: 'expense', amount: 3.84, note: 'ХимЛаб · 261115-1 · 2 шт', tx_id: '', src: 'wd_9' })
    expect(w.get('[data-testid="batch-left"]').text()).toBe('8')
    w.unmount()
  })

  it('Сжигание откатывается, если расход в деле не записался', async () => {
    docs.push(BATCH)
    const w = await mountLab()
    await openBatch(w)
    await w.get('[data-testid="move-burn"]').trigger('click')
    failEntries = true
    ;[...document.querySelectorAll('[data-testid="move-form"] button')].find((b) => b.textContent.includes('Сжечь')).click()
    await flushPromises()
    expect(calls.some((c) => c[0] === 'DELETE' && c[1].includes('doc=wd_9'))).toBe(true) // движение убрано
    expect(w.find('[data-testid="move-row"]').exists()).toBe(false)
    w.unmount()
  })

  it('Партия без штук не продаётся; удаление партии уносит её движения', async () => {
    docs.push({ ...BATCH, id: 'wd_c', data: { ...BATCH.data, pieces: 0, code: '261116-1' } })
    docs.push(BATCH, { id: 'wd_m', kind: 'move', day: '2026-11-16', data: { type: 'sale', batch_id: 'wd_b', pieces: 1, price: 10, note: '' } })
    const w = await mountLab()
    await w.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    const rows = w.findAll('[data-testid="batch"]')
    await rows[0].find('.group').trigger('click') // 261116-1, без штук
    expect(rows[0].text()).toContain('Укажи число штук')
    expect(rows[0].find('[data-testid="move-sale"]').exists()).toBe(false)
    await rows[1].find('[data-testid="batch-remove"]').trigger('click')
    await rows[1].find('[data-testid="batch-remove"]').trigger('click')
    await flushPromises()
    const del = calls.filter((c) => c[0] === 'DELETE').map((c) => c[1])
    expect(del.findIndex((u) => u.includes('doc=wd_m'))).toBeGreaterThanOrEqual(0)
    expect(del.findIndex((u) => u.includes('doc=wd_m'))).toBeLessThan(del.findIndex((u) => u.includes('doc=wd_b')))
    w.unmount()
  })
})

// Бейджи страницы «ХимЛаб»: расход (весы, посуда, химматериал — к химматериалу потом цепляются карточки товаров), убыток (ошибки, потери),
// продажа (приход от покупателей). Ключи уходят на бэкенд как есть. Итоги и наличные записи общие со страницей Glovo (utils/glovoLedger.js).
export const LAB_BADGES = [
  { key: 'expense', label: 'Расход', side: 'out', cls: 'bg-orange-500/15 text-orange-300 border-orange-500/30' },
  { key: 'loss', label: 'Убыток', side: 'out', cls: 'bg-rose-500/15 text-rose-300 border-rose-500/30' },
  { key: 'sale', label: 'Продажа', side: 'in', cls: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' }
]

// Карточки товаров. Карточка — конкретная закупка: вещество, производитель (необязательно), фасовка (нетто, г), цена за штуку и число штук.
// Цены не усредняются: одно вещество от двух производителей или в фасовках 100/500/1000 г остаётся разными карточками.
const r2 = (v) => Math.round(v * 100) / 100 + 0

export const CARD_KIND = 'card'

// карточка из полей формы: числа приведены, пустое вещество — null
export function cleanCard(f) {
  const n = (v) => Number(String(v ?? '').replace(',', '.'))
  const substance = String(f.substance || '').trim()
  const net_g = n(f.net_g)
  const price = n(f.price)
  const qty = n(f.qty)
  if (!substance || !(net_g > 0) || !(price >= 0) || !Number.isInteger(qty) || qty < 1) return null
  return { substance, maker: String(f.maker || '').trim(), net_g, price: r2(price), qty, note: String(f.note || '').trim() }
}

// стоимость и вес всей закупки по карточке, цена грамма (для себестоимости партий)
export const cardTotal = (c) => r2(c.price * c.qty)
export const cardGrams = (c) => c.net_g * c.qty
export const cardPerGram = (c) => (c.net_g > 0 ? c.price / c.net_g : 0)

// карточки по операциям: Map(id операции → карточки)
export function cardsByTx(cards) {
  const m = new Map()
  for (const c of cards) if (c.tx_id) m.set(c.tx_id, [...(m.get(c.tx_id) || []), c])
  return m
}

// сколько суммы операции разнесено по карточкам: { sum, diff } (diff — доставка и прочее, что не вошло в товары)
export function cardSplit(txAmount, list) {
  const sum = r2(list.reduce((s, c) => s + cardTotal(c), 0))
  return { sum, diff: r2(Math.abs(txAmount) - sum) }
}

// списки для подсказок формы: вещества и производители, которые уже вводились
export function cardNames(cards, field) {
  return [...new Set(cards.map((c) => String(c[field] || '').trim()).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'))
}

// короткая подпись карточки: «мальтодекстрин · Nutrimax · 1000 г»
export const cardTitle = (c) => [c.substance, c.maker, `${c.net_g} г`].filter(Boolean).join(' · ')

// Склад: запас по карточкам от номинала до расчётного. Номинал карточки — нетто × штуки; расход по ней (граммы из партий и списания)
// приходит в used: Map(id карточки → граммы). Пока партий нет, расчётный остаток равен номиналу. Цены карточек не усредняются:
// у вещества стоимость остатка — сумма по его карточкам, каждая по своей цене грамма.
export function stockModel(cards, used = new Map()) {
  const groups = new Map()
  for (const c of [...cards].sort((a, b) => (a.day < b.day ? -1 : a.day > b.day ? 1 : a.id < b.id ? -1 : 1))) {
    const key = String(c.substance).trim().toLowerCase()
    const nominal = cardGrams(c)
    const left = Math.min(nominal, Math.max(0, nominal - (used.get(c.id) || 0)))
    const row = { card: c, nominal, left, pct: nominal > 0 ? left / nominal : 0, valueTotal: cardTotal(c), valueLeft: r2(left * cardPerGram(c)) }
    if (!groups.has(key)) groups.set(key, { key, name: String(c.substance).trim(), rows: [], nominal: 0, left: 0, valueTotal: 0, valueLeft: 0 })
    const g = groups.get(key)
    g.rows.push(row)
    g.nominal += nominal
    g.left += left
    g.valueTotal = r2(g.valueTotal + row.valueTotal)
    g.valueLeft = r2(g.valueLeft + row.valueLeft)
  }
  const list = [...groups.values()].sort((a, b) => a.name.localeCompare(b.name, 'ru'))
  return {
    groups: list,
    cards: cards.length,
    valueTotal: r2(list.reduce((s, g) => s + g.valueTotal, 0)),
    valueLeft: r2(list.reduce((s, g) => s + g.valueLeft, 0))
  }
}

// Партии производства. Партия — документ вида batch: дата, номер ГГММДД-N, число штук (необязательно) и строки «карточка + граммы».
// Строка ссылается на конкретную карточку (цены не усредняются); вещество можно делить между несколькими карточками, если одна кончилась.
export const BATCH_KIND = 'batch'

// следующий номер партии за день: 2026-11-21 и две партии в этот день → «261121-3»
export function batchCode(day, batches) {
  const [y, m, d] = String(day).split('-')
  const base = `${String(y).slice(2)}${m}${d}`
  const n = batches.filter((b) => String(b.code).startsWith(`${base}-`)).length + 1
  return `${base}-${n}`
}

// партия из полей формы или null: нужны номер, дата и хотя бы одна строка с карточкой и граммами > 0
export function cleanBatch(f) {
  const n = (v) => Number(String(v ?? '').replace(',', '.'))
  const code = String(f.code || '').trim()
  const lines = (f.lines || []).map((l) => ({ card_id: String(l.card_id || ''), grams: r2(n(l.grams)) }))
  const pieces = f.pieces === '' || f.pieces == null ? 0 : n(f.pieces)
  if (!code || !/^\d{4}-\d{2}-\d{2}$/.test(f.day || '') || !lines.length) return null
  if (lines.some((l) => !l.card_id || !(l.grams > 0))) return null
  if (!Number.isInteger(pieces) || pieces < 0) return null
  return { day: f.day, code, pieces, note: String(f.note || '').trim(), lines }
}

// сколько граммов каждой карточки ушло в партии (except — партия, которую правят: её строки не считаются)
export function batchUsage(batches, except = '') {
  const m = new Map()
  for (const b of batches) {
    if (b.id === except) continue
    for (const l of b.lines || []) m.set(l.card_id, (m.get(l.card_id) || 0) + l.grams)
  }
  return m
}

// себестоимость партии: каждая строка по цене грамма своей карточки; удалённая карточка даёт missing и 0
export function batchCost(batch, cards) {
  const byId = new Map(cards.map((c) => [c.id, c]))
  const lines = (batch.lines || []).map((l) => {
    const card = byId.get(l.card_id) || null
    return { card, grams: l.grams, cost: card ? r2(l.grams * cardPerGram(card)) : 0, missing: !card }
  })
  const cost = r2(lines.reduce((s, l) => s + l.cost, 0))
  const grams = lines.reduce((s, l) => s + l.grams, 0)
  const pieces = batch.pieces || 0
  return { lines, cost, grams, perPiece: pieces ? r2(cost / pieces) : null, gramsPerPiece: pieces ? Math.round((grams / pieces) * 100) / 100 : null }
}

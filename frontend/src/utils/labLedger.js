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

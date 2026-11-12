// Таблица «Рынок»: фильтр, сортировка и подписи. Чистые функции над списком из /market/bonds.
import { isoToText } from './dateText.js'
import { curSym } from './formatters.js'

export const SOURCES = [
  { key: 'nbu', label: 'ОВДП', icon: 'bond', title: 'ОВДП · НБУ', idLabel: 'ISIN' },
  { key: 'ust', label: 'США', icon: 'usa', title: 'Облигации США', idLabel: 'CUSIP' }
]
export const MARKET_SECTIONS = SOURCES.map((s) => s.key)

// Колонки и способ сравнения; пустое значение всегда уходит в конец, в каком бы порядке ни сортировали
export const COLUMNS = [
  { key: 'id', label: 'ISIN', num: false },
  { key: 'name', label: 'Название', num: false },
  { key: 'maturity', label: 'Погашение', num: false },
  { key: 'coupon_rate', label: 'Ставка', num: true },
  { key: 'yield', label: 'Доходность', num: true },
  { key: 'last_auction', label: 'Аукцион', num: false },
  { key: 'nominal', label: 'Номинал', num: true }
]

const isEmpty = (v) => v === '' || v === null || v === undefined || (typeof v === 'number' && !(v > 0))

// Что значит «погашена»: дата погашения раньше сегодняшней. Без даты бумага считается действующей.
export const isMatured = (b, today) => !!b.maturity && b.maturity < today

export function filterBonds(list, { q = '', currency = '', hideMatured = true, today = '' } = {}) {
  const needle = String(q).trim().toUpperCase()
  return (Array.isArray(list) ? list : []).filter((b) => {
    if (currency && b.currency !== currency) return false
    if (hideMatured && today && isMatured(b, today)) return false
    if (needle && !String(b.id).toUpperCase().includes(needle) && !String(b.name || '').toUpperCase().includes(needle)) return false
    return true
  })
}

export function sortBonds(list, key, dir = 'asc') {
  const col = COLUMNS.find((c) => c.key === key)
  if (!col) return [...list]
  const sign = dir === 'desc' ? -1 : 1
  return [...list].sort((a, b) => {
    const x = a[key], y = b[key]
    const ex = isEmpty(x), ey = isEmpty(y)
    if (ex || ey) return ex === ey ? 0 : ex ? 1 : -1
    if (col.num) return (x - y) * sign
    return String(x).localeCompare(String(y), 'ru') * sign
  })
}

// Валюты, которые реально есть в списке (для чипов-фильтров), по алфавиту
export const currenciesOf = (list) => [...new Set((list || []).map((b) => b.currency).filter(Boolean))].sort()

// Округление до сотых: доходность приходит дробью, иначе в ячейке 15,299999999999999
const pct = (v) => (v > 0 ? `${String(Math.round(v * 100) / 100).replace('.', ',')} %` : '—')

// Подписи ячеек одной строки
export function bondCells(b) {
  return {
    id: b.id,
    name: b.name || '—',
    maturity: b.maturity ? isoToText(b.maturity) : '—',
    coupon_rate: pct(b.coupon_rate),
    yield: pct(b.yield),
    last_auction: b.last_auction ? isoToText(b.last_auction) : '—',
    nominal: b.nominal > 0 ? `${curSym(b.currency)} ${new Intl.NumberFormat('ru-RU').format(b.nominal)}` : '—'
  }
}

// Следующее направление сортировки при клике по заголовку: новый столбец — по возрастанию, тот же — наоборот
export const nextSort = (cur, key) => (cur.key === key ? { key, dir: cur.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' })

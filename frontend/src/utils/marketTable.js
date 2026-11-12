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
  { key: 'nominal', label: 'Номинал', num: true },
  { key: 'future', label: 'Через 10 л.', num: true },
  { key: 'score', label: 'Коэфф.', num: true }
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

// ── Коэффициент бумаги для долгосрочного накопления: чем выше, тем лучше бумага растит капитал за длинный горизонт.
// Модель: берём горизонт HORIZON_YEARS (10 лет). Бумага держит свою ставку r до погашения (купоны вкладываются под неё же),
// остаток горизонта деньги вкладываются под «базовую» ставку b — нижнюю четверть (25-й перцентиль) ставок действующих бумаг
// той же валюты: осторожное допущение «ставки со временем снизятся», а не «останутся как сейчас». Среднегодовой рост за горизонт:
//     g = ((1 + r)^T · (1 + b)^(H − T))^(1/H) − 1,  T = срок до погашения в годах (при T ≥ H просто g = r).
// Отсюда смысл: высокая ставка на ДОЛГИЙ срок = запертая выгода, лучшая; высокая на короткий — выгода кончится и придётся
// перекладываться (риск реинвестирования), чуть выше базовой; низкая на долгий срок — наоборот, ты запираешь плохую ставку
// и проигрываешь рынку. Коэффициент = место g между худшей (0 %) и лучшей (100 %) бумагой группы.
// Группа = одна валюта среди действующих бумаг: ставки в гривне, долларе и евро несравнимы. Ставка бумаги — аукционная
// доходность, а если её нет, то купонная. Погашенные, без даты погашения и без ставки не оцениваются.
// Считается по всему списку источника, а не по отфильтрованному: поиск не должен менять цвета и места.
// Упрощения: проценты годовые сложные (на деле купоны раз в полгода), налоги и инфляцию не учитывает; индексируемые
// на инфляцию ОВДП с низкой номинальной ставкой выглядят хуже, чем они есть на деле.
export const HORIZON_YEARS = 10

export const TIERS = [
  { key: 'top1', max: 1, label: 'Топ 1' },
  { key: 'top10', max: 10, label: 'Топ 10' },
  { key: 'top25', max: 25, label: 'Топ 25' },
  { key: 'top100', max: 100, label: 'Топ 100' }
]

export const rateOf = (b) => (b.yield > 0 ? b.yield : b.coupon_rate > 0 ? b.coupon_rate : 0)

const dayNum = (iso) => Date.parse(`${iso}T00:00:00Z`) / 86400000

export const REINVEST_QUANTILE = 0.25

// Квантиль с линейной интерполяцией между соседними значениями
export const quantile = (xs, q) => {
  const v = [...xs].sort((x, y) => x - y)
  const pos = (v.length - 1) * q
  const i = Math.floor(pos)
  return i + 1 < v.length ? v[i] + (v[i + 1] - v[i]) * (pos - i) : v[i]
}

// Среднегодовой рост за горизонт: ставка r (в %) держится years лет, остаток — под базовую base (в %)
export function growthOf(r, years, base, horizon = HORIZON_YEARS) {
  if (years >= horizon) return r
  const held = Math.log1p(r / 100) * years + Math.log1p(base / 100) * (horizon - years)
  return (Math.expm1(held / horizon)) * 100
}

export function scoreBonds(list, today) {
  const t0 = dayNum(today)
  const groups = new Map()
  for (const b of Array.isArray(list) ? list : []) {
    if (!b.maturity || isMatured(b, today)) continue
    const rate = rateOf(b)
    if (!(rate > 0)) continue
    const years = Math.max(0, dayNum(b.maturity) - t0) / 365.25
    if (!groups.has(b.currency)) groups.set(b.currency, [])
    groups.get(b.currency).push({ id: b.id, currency: b.currency, years, rate })
  }
  const out = new Map()
  for (const [currency, items] of groups) {
    const base = quantile(items.map((i) => i.rate), REINVEST_QUANTILE)
    const grown = items.map((i) => ({ ...i, base, growth: growthOf(i.rate, i.years, base) }))
    const lo = Math.min(...grown.map((i) => i.growth))
    const hi = Math.max(...grown.map((i) => i.growth))
    const scored = grown.map((i) => ({ ...i, score: hi > lo ? ((i.growth - lo) / (hi - lo)) * 100 : 100 }))
    scored.sort((a, b) => b.score - a.score || a.id.localeCompare(b.id))
    scored.forEach((s, i) => {
      const rank = i + 1
      const tier = TIERS.find((t) => rank <= t.max) || null
      // во сколько раз вырастет вложение: к погашению (ставка бумаги) и за весь горизонт (с перекладыванием остатка)
      const multipleT = Math.pow(1 + s.rate / 100, s.years)
      const multipleH = Math.pow(1 + s.growth / 100, HORIZON_YEARS)
      out.set(s.id, { ...s, rank, multipleT, multipleH, tier: tier ? tier.key : '', tierLabel: tier ? tier.label : '' })
    })
  }
  return out
}

// Цвет от красного (0 %) через жёлтый (50 %) к зелёному (100 %): тон 0°…120° в HSL
export function scoreColor(score) {
  const h = Math.round(Math.min(100, Math.max(0, score)) * 1.2)
  return { border: `hsl(${h} 70% 45% / 0.6)`, bg: `hsl(${h} 70% 40% / 0.12)`, text: `hsl(${h} 75% 65%)` }
}

// Сумма с символом валюты: ₴ 12 345
export const moneyText = (v, currency) => `${curSym(currency)}\u00A0${new Intl.NumberFormat('ru-RU').format(Math.round(v))}`

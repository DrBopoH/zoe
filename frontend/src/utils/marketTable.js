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
  { key: 'real', label: 'Реал.', num: true, signed: true }, // доходность минус инфляция; бывает отрицательной и нулевой, это не «пусто»
  { key: 'usd', label: 'В $', num: true, signed: true }, // доходность в долларах: доллар как есть, гривна и евро через курс
  { key: 'realUsd', label: 'Реал. $', num: true, signed: true }, // то же с поправкой на инфляцию США
  { key: 'last_auction', label: 'Аукцион', num: false },
  { key: 'nominal', label: 'Номинал', num: true },
  { key: 'future', label: '10 л.', num: true },
  { key: 'score', label: 'Коэфф.', num: true }
]

const isEmpty = (v) => v === '' || v === null || v === undefined || (typeof v === 'number' && !(v > 0))

// Что значит «погашена»: дата погашения раньше сегодняшней. Без даты бумага считается действующей.
// Список доступных в брокере (ICU) бумаг: из вставленного текста берём ISIN (UA + 10 цифр), без повторов
export const parseIsins = (text) => [...new Set(String(text || '').toUpperCase().match(/UA\d{10}/g) || [])]
// Оставить только бумаги из набора ISIN; пустой набор не фильтрует (иначе «только ICU» без списка обнулил бы таблицу)
export const restrictTo = (list, ids) => (!list || !ids || !ids.size ? list : list.filter((b) => ids.has(String(b.id).toUpperCase())))

// Сколько штук каждой бумаги уже куплено: позиции брокерских счетов (ответы /icu/ledger), по ISIN, счета складываются
export function holdingsOf(overviews) {
  const out = new Map()
  for (const ov of Array.isArray(overviews) ? overviews : []) {
    for (const p of (ov && ov.positions) || []) {
      const id = String(p.isin || '').toUpperCase()
      const qty = Number(p.qty)
      if (id && qty > 0) out.set(id, (out.get(id) || 0) + qty)
    }
  }
  return out
}
export const qtyText = (n) => `${new Intl.NumberFormat('ru-RU').format(Math.round(n * 100) / 100)}\u00A0шт.`

// Из вставленного текста (экран брокера, распознанный скриншот): ISIN и цена за бумагу, если она есть сразу после ISIN
// (до следующего ISIN). Цена — число с двумя знаками от 100 до 100 000: проценты (YTM 16.15), даты и количества не подходят.
export function parseIcuList(text) {
  const src = String(text || '').toUpperCase()
  const hits = [...src.matchAll(/UA\d{10}/g)]
  const isins = [], prices = {}
  hits.forEach((m, i) => {
    if (!isins.includes(m[0])) isins.push(m[0])
    const seg = src.slice(m.index + 12, i + 1 < hits.length ? hits[i + 1].index : undefined)
    const num = [...seg.matchAll(/(\d{1,3}(?:[\s\u00A0]\d{3})+|\d+)[.,](\d{2})(?!\d)/g)]
      .map((x) => Number(`${x[1].replace(/[\s\u00A0]/g, '')}.${x[2]}`))
      .find((v) => v >= 100 && v <= 100000)
    if (num !== undefined && prices[m[0]] === undefined) prices[m[0]] = num
  })
  return { isins, prices }
}

const addMonthsMs = (ms, n) => {
  const d = new Date(ms), day = d.getUTCDate()
  const x = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + n, 1))
  x.setUTCDate(Math.min(day, new Date(Date.UTC(x.getUTCFullYear(), x.getUTCMonth() + 1, 0)).getUTCDate()))
  return x.getTime()
}

// Кривая доходности по опорным точкам [{ years, ytm }]: между точками линейно, за краями — ближайшее значение; null, если точек нет.
export function curveAt(points, years) {
  const pts = (Array.isArray(points) ? points : []).filter((p) => p && p.years > 0 && p.ytm > 0).sort((a, b) => a.years - b.years)
  if (!pts.length || !Number.isFinite(years)) return null
  if (years <= pts[0].years) return pts[0].ytm
  const last = pts[pts.length - 1]
  if (years >= last.years) return last.ytm
  for (let i = 1; i < pts.length; i++) {
    if (years <= pts[i].years) {
      const a = pts[i - 1], b = pts[i]
      return b.years === a.years ? b.ytm : a.ytm + ((b.ytm - a.ytm) * (years - a.years)) / (b.years - a.years)
    }
  }
  return last.ytm
}

/**
 * Доходность каждой бумаги из списка брокера. Приоритет: по цене (price) → показанная брокером (ytm) → оценка по кривой (curve).
 * Кривая строится из бумаг с известной доходностью и ваших точек (curve). Без всего этого бумаги в карту не попадают.
 * Возвращает Map ISIN → { rate, source: 'price' | 'ytm' | 'curve' }.
 */
export function resolveRates({ pool, prices = {}, ytms = {}, curve = [], today }) {
  const t0 = Date.parse(`${today}T00:00:00Z`)
  const yearsOf = (b) => (Date.parse(`${b.maturity}T00:00:00Z`) - t0) / (365.25 * 86400000)
  const out = new Map()
  const anchors = (Array.isArray(curve) ? curve : []).map((p) => ({ years: p.years, ytm: p.ytm }))
  const unknown = []
  for (const b of Array.isArray(pool) ? pool : []) {
    if (!b.maturity || isMatured(b, today)) continue
    const id = String(b.id).toUpperCase()
    const price = prices[id]
    const fromPrice = price > 0 ? ytmFromPrice(b, price, today) : null
    if (fromPrice > 0) { out.set(id, { rate: fromPrice, source: 'price' }); anchors.push({ years: yearsOf(b), ytm: fromPrice }) }
    else if (ytms[id] > 0) { out.set(id, { rate: ytms[id], source: 'ytm' }); anchors.push({ years: yearsOf(b), ytm: ytms[id] }) }
    else unknown.push({ id, years: yearsOf(b) })
  }
  for (const u of unknown) {
    const y = curveAt(anchors, u.years)
    if (y > 0) out.set(u.id, { rate: y, source: 'curve' })
  }
  return out
}

// Доходность к погашению (эффективная годовая, %) по цене из брокера. Цена — полная, с накопленным купоном (как в ICU).
// Купоны раз в полгода, от даты погашения назад; тот же порядок, что у расчёта выплат на бэкенде (couponDays). Без купона —
// одна выплата в день погашения. null — если бумага погашена, цены нет или решения нет.
export function ytmFromPrice(b, price, today) {
  const nominal = b.nominal > 0 ? b.nominal : 1000
  if (!b.maturity || !(price > 0) || isMatured(b, today)) return null
  const t0 = Date.parse(`${today}T00:00:00Z`), mat = Date.parse(`${b.maturity}T00:00:00Z`)
  if (!(mat > t0)) return null
  const coupon = b.coupon_rate > 0 ? (nominal * b.coupon_rate) / 100 / 2 : 0
  const flows = []
  if (coupon > 0) for (let k = 0, d = mat; d > t0 && k < 200; k++, d = addMonthsMs(mat, -6 * k)) flows.push({ t: (d - t0) / 86400000 / 365, cf: coupon })
  else flows.push({ t: (mat - t0) / 86400000 / 365, cf: 0 })
  flows[0].cf += nominal
  const pv = (y) => flows.reduce((s, f) => s + f.cf / Math.pow(1 + y, f.t), 0)
  let lo = -0.9, hi = 5
  if (!(pv(lo) > price) || !(pv(hi) < price)) return null
  for (let i = 0; i < 100; i++) { const mid = (lo + hi) / 2; if (pv(mid) > price) lo = mid; else hi = mid }
  return ((lo + hi) / 2) * 100
}

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
    const empty = col.signed ? (v) => v === null || v === undefined || Number.isNaN(v) : isEmpty
    const ex = empty(x), ey = empty(y)
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

export function scoreBonds(list, today, rates = null) {
  const t0 = dayNum(today)
  const groups = new Map()
  for (const b of Array.isArray(list) ? list : []) {
    if (!b.maturity || isMatured(b, today)) continue
    const own = rates && rates.get(String(b.id).toUpperCase()) // своя доходность (например из цены брокера) важнее аукционной и купонной
    const rate = own > 0 ? own : rateOf(b)
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

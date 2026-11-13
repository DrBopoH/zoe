import { scaleTicks, timeTicks } from './growthChart.js'
import { robustRanges, scaleOf } from './chartView.js'

// Макро НБУ: ключевые ставки и инфляция (ряды из /api/macro). Чистые функции — без интерфейса.
// Ставки приходят датами смены значения (значение действует до следующей точки), инфляция — помесячно.

export const SERIES = {
  policy_rate: { label: 'Облікова ставка', color: '#34d399', step: true },
  cpi_yoy: { label: 'Инфляция г/г', color: '#fbbf24', step: false },
  core_yoy: { label: 'Базовая инфляция г/г', color: '#fb923c', step: false, thin: true },
  overnight_cd: { label: 'Сертификаты овернайт', color: '#38bdf8', step: true },
  overnight_loans: { label: 'Кредиты овернайт', color: '#a78bfa', step: true },
  cpi_mom: { label: 'Инфляция м/м', color: '#fbbf24', step: false },
  core_mom: { label: 'Базовая м/м', color: '#fb923c', step: false, thin: true },
  usd_uah: { label: 'Доллар США', color: '#34d399', step: false },
  eur_uah: { label: 'Евро', color: '#38bdf8', step: false },
  xau_uah: { label: 'Золото, ₴ за унцию', color: '#fbbf24', step: false },
  xau_usd: { label: 'Золото, $ за унцию', color: '#fbbf24', step: false },
  fed_funds: { label: 'Ставка ФРС (факт)', color: '#a78bfa', step: false },
  fed_target: { label: 'ФРС: верх коридора', color: '#34d399', step: true },
  ust_10y: { label: 'Treasury 10 л.', color: '#38bdf8', step: false, thin: true },
  eu_cpi_yoy: { label: 'Инфляция еврозоны г/г', color: '#38bdf8', step: false },
  us_cpi_yoy: { label: 'Инфляция США г/г', color: '#fbbf24', step: false },
  us_cpi_mom: { label: 'Инфляция США м/м', color: '#fb923c', step: false },
  us_unrate: { label: 'Безработица США', color: '#38bdf8', step: false },
  us_gdp: { label: 'Рост ВВП США (в год)', color: '#34d399', step: true },
  us_spread: { label: 'Кривая: 10 л. − 2 г.', color: '#a78bfa', step: false },
  // как у индекса страха и жадности: середина (около 20, обычный фон) полупрозрачный серый, высокий VIX = страх голубой, очень низкий = самоуспокоенность янтарная
  us_vix: { label: 'VIX (индекс страха)', color: '#94a3b8', step: false, gradient: { from: 10, to: 50, stops: [[0, '#fbbf24', 1], [0.25, '#94a3b8', 0.4], [0.75, '#38bdf8', 1], [1, '#38bdf8', 1]] } },
  // светофор: чем оптимистичнее люди, тем зеленее (от 50 красный, около 75 жёлтый, от 100 зелёный)
  us_sentiment: { label: 'Настроения потребителей', color: '#fbbf24', step: false, noLog: true, gradient: { from: 50, to: 100, stops: [[0, '#f87171', 1], [0.5, '#fbbf24', 1], [1, '#34d399', 1]] } },
  crypto_btc: { label: 'Bitcoin', color: '#fb923c', step: false },
  crypto_eth: { label: 'Ethereum', color: '#a78bfa', step: false },
  crypto_bnb: { label: 'BNB', color: '#fbbf24', step: false },
  crypto_sol: { label: 'Solana', color: '#34d399', step: false },
  // gradient: цвет линии зависит от значения (stops: [доля от from до to, цвет, прозрачность]); noLog — ограниченный индекс, логарифм не нужен
  crypto_fng: { label: 'Страх и жадность', color: '#94a3b8', step: false, noLog: true, gradient: { from: 0, to: 100, stops: [[0, '#38bdf8', 1], [0.5, '#94a3b8', 0.4], [1, '#fbbf24', 1]] } },
  stock_sp500: { label: 'S&P 500', color: '#38bdf8', step: false },
  stock_nasdaq: { label: 'NASDAQ Composite', color: '#a78bfa', step: false },
  stock_dow: { label: 'Dow Jones', color: '#fbbf24', step: false },
  stock_sp500_dd: { label: 'S&P 500: просадка от максимума', color: '#38bdf8', step: false },
  stock_nasdaq_dd: { label: 'NASDAQ: просадка от максимума', color: '#a78bfa', step: false },
  stock_dow_dd: { label: 'Dow Jones: просадка от максимума', color: '#fbbf24', step: false },
  crypto_btc_dd: { label: 'Bitcoin: просадка от максимума', color: '#fb923c', step: false },
  crypto_eth_dd: { label: 'Ethereum: просадка от максимума', color: '#a78bfa', step: false }
}

// Число для оси и подсказки по единице ряда: «%» — процент, «₴»/«$» — деньги (тысячи сокращаются: 183 562 → «184 тыс.»)
export function unitText(v, unit = '%') {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  if (unit === '%') return plainPct(v)
  const abs = Math.abs(v)
  const body = abs >= 10000 ? `${Math.round(v / 1000)} тыс.` : abs >= 100 ? String(Math.round(v)) : String(Math.round(v * 100) / 100).replace('.', ',')
  return `${body} ${unit}`
}
// Метка оси: то же, но короче (без десятых у дробных, чтобы подписи не слипались)
export const axisText = (v, unit = '%') => {
  if (unit === '%') return `${v} %`
  if (Math.abs(v) >= 10000) return `${Math.round(v / 1000)}к ${unit}`
  return unitText(v, unit)
}

// Золото в долларах: гривенная цена унции на курс доллара той же даты (курс — последний не позже даты)
export function goldUsd(series) {
  const g = series && series.xau_uah, u = series && series.usd_uah
  if (!g || !u || !g.length || !u.length) return []
  const out = []
  for (const p of g) {
    const r = valueAt(u, p.d)
    if (r) out.push({ d: p.d, v: p.v / r })
  }
  return out
}

// Среднегодовое изменение ряда за последние years лет, % (сложный процент). Нет данных так далеко назад — null.
export function annualChange(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  if (then === null || !(then > 0) || !(last.v > 0)) return null
  return (Math.pow(last.v / then, 1 / years) - 1) * 100
}

const DAY = 86400000
const ms = (iso) => Date.parse(`${iso}T00:00:00Z`)

// Значение ряда на дату: последняя точка не позже даты; раньше первой точки — null. Ряд отсортирован по возрастанию.
export function valueAt(points, iso) {
  if (!points || !points.length || !iso || iso < points[0].d) return null
  let lo = 0, hi = points.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (points[mid].d <= iso) lo = mid
    else hi = mid - 1
  }
  return points[lo].v
}

// Последняя точка ряда или null
export const lastPoint = (points) => (points && points.length ? points[points.length - 1] : null)

// «2026-11-13» → «13.11.2026»; пусто для пустой даты
export const dateOf = (iso) => (iso ? iso.split('-').reverse().join('.') : '')
// Есть ли хоть одна точка хоть в одном ряду (пока нет — на экране подсказка «Нажми Обновить»)
export const hasPoints = (series) => Object.values(series || {}).some((a) => a && a.length)

// Реальная доходность, % (по Фишеру): (1 + номинальная) / (1 + инфляция) − 1. Нет инфляции или ставки — null.
export function realRate(nominal, inflation) {
  if (!(typeof nominal === 'number' && isFinite(nominal)) || !(typeof inflation === 'number' && isFinite(inflation))) return null
  return ((1 + nominal / 100) / (1 + inflation / 100) - 1) * 100
}

/** Инфляция г/г (последний известный месяц) в валюте бумаги: UAH — НБУ, USD — США, EUR — еврозона (HICP). Нет данных или чужая валюта — null. */
export function inflationFor(currency, series) {
  const id = { UAH: 'cpi_yoy', USD: 'us_cpi_yoy', EUR: 'eu_cpi_yoy' }[currency]
  return id ? lastPoint(series && series[id])?.v ?? null : null
}

// Текущая инфляция для расчёта реальной доходности: последний известный месяц (г/г). null — данных нет.
export const currentInflation = (series) => lastPoint(series && series.cpi_yoy)?.v ?? null

// Знаковый процент с запятой: «+9,4 %», «−1,2 %», «0 %»
export function signedPct(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const r = Math.round(v * 10) / 10
  const s = String(Math.abs(r)).replace('.', ',')
  return `${r > 0 ? '+' : r < 0 ? '−' : ''}${s} %`
}
export const plainPct = (v) => (v === null || v === undefined || !isFinite(v) ? '—' : `${String(Math.round(v * 100) / 100).replace('.', ',')} %`)

// Сводка для шапки блока: текущие значения и разрыв «ставка − инфляция»
export function macroSummary(series, todayIso) {
  const at = (id) => (series && series[id] ? valueAt(series[id], todayIso) : null)
  const policy = at('policy_rate')
  const cpi = lastPoint(series && series.cpi_yoy)
  return {
    policy,
    overnightCD: at('overnight_cd'),
    cpi: cpi ? cpi.v : null,
    cpiMonth: cpi ? cpi.d.slice(0, 7) : '',
    core: lastPoint(series && series.core_yoy)?.v ?? null,
    realPolicy: policy !== null && cpi ? realRate(policy, cpi.v) : null
  }
}

export const MACRO_PAD = { l: 56, r: 16, t: 12, b: 36 } // поля графика (px во viewBox): по ним мышь переводится в данные
export const MACRO_MIN_SPAN = 0.5 // дальше приближать смысла нет: полгода
export const MACRO_MAX_SPAN = 60

const YEAR_MS = 365.25 * DAY
// Годы от сегодня (отрицательные — прошлое): та же ось X, что у графика роста, поэтому и общий движок (composables/useChartView.js)
export const yearsFrom = (iso, todayIso) => (ms(iso) - ms(todayIso)) / YEAR_MS
export const isoAt = (years, todayIso) => new Date(ms(todayIso) + years * YEAR_MS).toISOString().slice(0, 10)

const usable = (series, ids) => (ids || []).filter((id) => series && series[id] && series[id].length)

/** Окно по умолчанию: years лет назад (0 — с первой точки, весь период), до сегодня с небольшим запасом справа. Левее первой точки данных не уходит. */
export function macroWindow({ series, ids, todayIso, years = 0 }) {
  const use = usable(series, ids)
  if (!use.length) return { x0: -1, x1: 0.05 }
  const first = use.map((id) => series[id][0].d).sort()[0]
  const firstT = yearsFrom(first, todayIso)
  const left = years ? Math.max(-years, firstT) : firstT
  const span = Math.max(-left, MACRO_MIN_SPAN)
  return { x0: -span, x1: span * 0.02 }
}

/** Вертикаль: auto — под видимое окно без редких выбросов (один сбойный месяц не должен сплющивать график), ext — крайности по всем данным. */
export function macroRanges({ series, ids, view, todayIso, floor = 0, log = false }) {
  const visible = [], all = []
  for (const id of usable(series, ids)) {
    const pts = series[id]
    const before = valueAt(pts, isoAt(view.x0, todayIso))
    if (before !== null) visible.push(before)
    for (const p of pts) {
      all.push(p.v)
      const t = yearsFrom(p.d, todayIso)
      if (t >= view.x0 && t <= view.x1) visible.push(p.v)
    }
  }
  return robustRanges({ visible, all, floor, log })
}

/**
 * График: ставка ступенькой (держится до следующей смены, последнее значение тянется до сегодня), инфляция ломаной.
 * view — окно { x0, x1 } в годах от сегодня, yRange — { lo, hi }. Точки за краями остаются в линии, обрезает рамка графика.
 * Возвращает { lines:[{id,color,label,d}], yTicks, xTicks, todayX, zeroY, empty, pad, width, height }.
 */
export function buildMacroChart({ series, ids, view, yRange, todayIso, width = 1000, height = 220, pad = MACRO_PAD, scale = 'linear' }) {
  const use = usable(series, ids)
  if (!use.length) return { lines: [], yTicks: [], xTicks: [], todayX: 0, zeroY: null, empty: true, pad, width, height }
  const { x0, x1 } = view
  const { lo, hi } = yRange
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * (width - pad.l - pad.r)
  const sc = scaleOf(scale)
  // yRange — в шкальном пространстве (при log это логарифмы); значения ≤ 0 на логарифмической оси не рисуются (разрыв линии)
  const Y = (v) => pad.t + (1 - (sc.fwd(v) - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const margin = (x1 - x0) * 0.05

  const lines = use.map((id) => {
    const meta = SERIES[id] || { label: id, color: '#94a3b8', step: false }
    // точки окна с запасом и одна точка перед ним (чтобы линия входила с левого края)
    const pts = []
    let prev = null
    for (const p of series[id]) {
      if (sc.mode === 'log' && !(p.v > 0)) continue
      const t = yearsFrom(p.d, todayIso)
      if (t < x0 - margin) { prev = { t: x0 - margin, v: p.v }; continue }
      if (t > x1 + margin) break
      if (prev) { pts.push(prev); prev = null }
      pts.push({ t, v: p.v })
    }
    if (prev) pts.push(prev)
    let d = ''
    pts.forEach((p, i) => {
      const x = X(p.t).toFixed(1), y = Y(p.v).toFixed(1)
      d += i === 0 ? `M${x} ${y}` : meta.step ? `H${x}V${y}` : `L${x} ${y}`
    })
    if (meta.step && pts.length) d += `H${X(Math.min(x1 + margin, 0)).toFixed(1)}`
    // градиент по значению: вектор от значения from до to в пикселях графика (userSpaceOnUse), поэтому цвет привязан к уровню, а не к положению на экране
    const g = meta.gradient ? { y1: Y(meta.gradient.from), y2: Y(meta.gradient.to), stops: meta.gradient.stops.map(([offset, color, opacity]) => ({ offset, color, opacity })) } : null
    return { id, color: meta.color, label: meta.label, d, gradient: g && isFinite(g.y1) && isFinite(g.y2) && g.y1 !== g.y2 ? g : null }
  })

  return {
    lines,
    yTicks: scaleTicks(sc.mode, lo, hi, 5).map((v) => ({ v, y: Y(v) })),
    xTicks: timeTicks(x0, x1, todayIso).map((t) => ({ x: X(t.t), label: t.label })),
    todayX: X(0),
    zeroY: sc.mode === 'linear' && lo < 0 && hi > 0 ? Y(0) : null,
    pad, width, height,
    empty: false
  }
}

// ── Экономика США: сигналы по рядам FRED (чистые функции; подсказки на экране, не советы).

/**
 * Правило Сам (Sahm): средняя безработица за 3 месяца минус её минимум за последние 12 месяцев (включая текущий).
 * Рост на 0,5 п. п. и больше исторически совпадал с началом рецессии. Ряд — помесячные точки по возрастанию; меньше 14 точек — null.
 * Возвращает { value, signal } (value в п. п.).
 */
export function sahmRule(points) {
  const v = (points || []).map((p) => p.v)
  if (v.length < 14) return null
  const avg = []
  for (let i = 2; i < v.length; i++) avg.push((v[i] + v[i - 1] + v[i - 2]) / 3)
  const last = avg[avg.length - 1]
  const min = Math.min(...avg.slice(-12))
  const value = Math.round((last - min) * 100) / 100
  return { value, signal: value >= 0.5 }
}

/** Два квартала подряд спада ВВП — «техническая рецессия». Ряд поквартальный; false, если меньше двух точек. */
export function technicalRecession(points) {
  const n = (points || []).length
  return n >= 2 && points[n - 1].v < 0 && points[n - 2].v < 0
}

/** Состояние кривой доходности (10 л. − 2 г., п. п.): отрицательная — «инвертирована» (исторически опережала рецессии). null — нет данных. */
export const curveState = (spread) => (spread === null || spread === undefined || !isFinite(spread) ? null : spread < 0 ? 'inverted' : spread < 0.5 ? 'flat' : 'normal')

/** Уровень VIX словами: до 20 спокойно, 20–30 тревожно, выше 30 паника. */
export const vixLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 20 ? 'calm' : v <= 30 ? 'nervous' : 'panic')

// ── Крипто: просадка от максимума, рост за период, индекс страха и жадности словами (чистые функции).

/** Просадка от исторического максимума, %: 0 на максимуме, −50 — цена вдвое ниже пика. Ряд по возрастанию дат. */
export function drawdown(points) {
  let peak = 0
  const out = []
  for (const p of points || []) {
    if (!(p.v > 0)) continue
    if (p.v > peak) peak = p.v
    out.push({ d: p.d, v: (p.v / peak - 1) * 100 })
  }
  return out
}

/** Максимум за всю историю и где цена сейчас относительно него: { ath, date, fromAth (%, ≤ 0) }. Нет данных — null. */
export function athInfo(points) {
  const pts = (points || []).filter((p) => p.v > 0)
  if (!pts.length) return null
  let top = pts[0]
  for (const p of pts) if (p.v > top.v) top = p
  const last = pts[pts.length - 1]
  return { ath: top.v, date: top.d, fromAth: (last.v / top.v - 1) * 100 }
}

/** Изменение цены за years лет назад (по последней точке не позже даты), % — не среднегодовое. Нет данных так далеко — null. */
export function changeSince(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  return then > 0 && last.v > 0 ? (last.v / then - 1) * 100 : null
}

/** Индекс страха и жадности словами: до 25 крайний страх, до 45 страх, до 55 нейтрально, до 75 жадность, выше крайняя жадность. */
export const fearGreedLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 25 ? 'extreme-fear' : v < 45 ? 'fear' : v <= 55 ? 'neutral' : v <= 75 ? 'greed' : 'extreme-greed')

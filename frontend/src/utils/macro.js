import { niceTicks, timeTicks } from './growthChart.js'
import { robustRanges } from './chartView.js'

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
  us_cpi_yoy: { label: 'Инфляция США г/г', color: '#fbbf24', step: false },
  us_cpi_mom: { label: 'Инфляция США м/м', color: '#fb923c', step: false }
}

// Ряды по группам: для запроса /macro (ставки и инфляция — малые, курсы — тысячи точек, грузятся отдельно)
export const MACRO_IDS = ['policy_rate', 'overnight_cd', 'overnight_loans', 'cpi_yoy', 'cpi_mom', 'core_yoy', 'core_mom']
export const FX_IDS = ['usd_uah', 'eur_uah', 'xau_uah']
export const US_IDS = ['fed_funds', 'fed_target', 'ust_10y', 'us_cpi_yoy', 'us_cpi_mom']

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

// Реальная доходность, % (по Фишеру): (1 + номинальная) / (1 + инфляция) − 1. Нет инфляции или ставки — null.
export function realRate(nominal, inflation) {
  if (!(typeof nominal === 'number' && isFinite(nominal)) || !(typeof inflation === 'number' && isFinite(inflation))) return null
  return ((1 + nominal / 100) / (1 + inflation / 100) - 1) * 100
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
export function macroRanges({ series, ids, view, todayIso, floor = 0 }) {
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
  return robustRanges({ visible, all, floor })
}

/**
 * График: ставка ступенькой (держится до следующей смены, последнее значение тянется до сегодня), инфляция ломаной.
 * view — окно { x0, x1 } в годах от сегодня, yRange — { lo, hi }. Точки за краями остаются в линии, обрезает рамка графика.
 * Возвращает { lines:[{id,color,label,d}], yTicks, xTicks, todayX, zeroY, empty, pad, width, height }.
 */
export function buildMacroChart({ series, ids, view, yRange, todayIso, width = 1000, height = 220, pad = MACRO_PAD }) {
  const use = usable(series, ids)
  if (!use.length) return { lines: [], yTicks: [], xTicks: [], todayX: 0, zeroY: null, empty: true, pad, width, height }
  const { x0, x1 } = view
  const { lo, hi } = yRange
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * (width - pad.l - pad.r)
  const Y = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const margin = (x1 - x0) * 0.05

  const lines = use.map((id) => {
    const meta = SERIES[id] || { label: id, color: '#94a3b8', step: false }
    // точки окна с запасом и одна точка перед ним (чтобы линия входила с левого края)
    const pts = []
    let prev = null
    for (const p of series[id]) {
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
    return { id, color: meta.color, label: meta.label, d }
  })

  return {
    lines,
    yTicks: niceTicks(lo, hi, 5).map((v) => ({ v, y: Y(v) })),
    xTicks: timeTicks(x0, x1, todayIso).map((t) => ({ x: X(t.t), label: t.label })),
    todayX: X(0),
    zeroY: lo < 0 && hi > 0 ? Y(0) : null,
    pad, width, height,
    empty: false
  }
}

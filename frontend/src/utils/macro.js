import { niceTicks } from './growthChart.js'

// Макро НБУ: ключевые ставки и инфляция (ряды из /api/macro). Чистые функции — без интерфейса.
// Ставки приходят датами смены значения (значение действует до следующей точки), инфляция — помесячно.

export const SERIES = {
  policy_rate: { label: 'Облікова ставка', color: '#34d399', step: true },
  cpi_yoy: { label: 'Инфляция г/г', color: '#fbbf24', step: false },
  core_yoy: { label: 'Базовая инфляция г/г', color: '#fb923c', step: false },
  overnight_cd: { label: 'Сертификаты овернайт', color: '#38bdf8', step: true }
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

// Диапазоны окна графика: сколько лет назад начинать (0 — с первой точки)
export const RANGES = [
  { key: '5', label: '5 л.', years: 5 },
  { key: '10', label: '10 л.', years: 10 },
  { key: 'all', label: 'Всё', years: 0 }
]

const startOf = (years, firstIso, todayIso) => {
  if (!years) return firstIso
  const d = new Date(ms(todayIso))
  d.setUTCFullYear(d.getUTCFullYear() - years)
  const iso = d.toISOString().slice(0, 10)
  return iso > firstIso ? iso : firstIso
}

// График: ставка ступенькой (держится до следующей смены, последнее значение тянется до сегодня), инфляция ломаной.
// Возвращает { lines:[{id,color,label,d}], yTicks:[{v,y}], xTicks:[{x,label}], todayX, zeroY, empty }.
export function buildMacroChart({ series, ids, years, todayIso, width = 1000, height = 240, pad = { l: 46, r: 16, t: 12, b: 24 } }) {
  const use = (ids || []).filter((id) => series && series[id] && series[id].length)
  if (!use.length) return { lines: [], yTicks: [], xTicks: [], todayX: 0, zeroY: null, empty: true }
  const first = use.map((id) => series[id][0].d).sort()[0]
  const from = startOf(years, first, todayIso)
  const t0 = ms(from), t1 = ms(todayIso)
  const span = Math.max(t1 - t0, DAY)

  // значения в окне, чтобы подобрать шкалу Y
  const vals = []
  const clipped = {}
  for (const id of use) {
    const pts = series[id]
    const before = valueAt(pts, from)
    const inside = pts.filter((p) => p.d >= from && p.d <= todayIso)
    clipped[id] = { before, inside }
    if (before !== null) vals.push(before)
    for (const p of inside) vals.push(p.v)
  }
  let lo = Math.min(0, ...vals), hi = Math.max(...vals)
  if (!(hi > lo)) hi = lo + 1
  const ticks = niceTicks(lo, hi, 5)
  lo = Math.min(lo, ticks[0])
  hi = Math.max(hi, ticks[ticks.length - 1])
  const X = (iso) => pad.l + ((ms(iso) - t0) / span) * (width - pad.l - pad.r)
  const Y = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * (height - pad.t - pad.b)

  const lines = use.map((id) => {
    const meta = SERIES[id] || { label: id, color: '#94a3b8', step: false }
    const { before, inside } = clipped[id]
    const pts = []
    if (before !== null && (!inside.length || inside[0].d > from)) pts.push({ d: from, v: before })
    pts.push(...inside)
    let d = ''
    pts.forEach((p, i) => {
      const x = X(p.d), y = Y(p.v)
      if (i === 0) d += `M${x.toFixed(1)} ${y.toFixed(1)}`
      else if (meta.step) d += `H${x.toFixed(1)}V${y.toFixed(1)}`
      else d += `L${x.toFixed(1)} ${y.toFixed(1)}`
    })
    if (meta.step && pts.length) d += `H${X(todayIso).toFixed(1)}`
    return { id, color: meta.color, label: meta.label, d }
  })

  // Ось времени: годы, шаг подбирается под ширину окна
  const y0 = Number(from.slice(0, 4)) + (from.slice(5) === '01-01' ? 0 : 1)
  const y1 = Number(todayIso.slice(0, 4))
  const step = Math.max(1, Math.ceil((y1 - y0 + 1) / 8))
  const xTicks = []
  for (let y = y0; y <= y1; y += step) xTicks.push({ x: X(`${y}-01-01`), label: String(y) })

  return {
    lines,
    yTicks: ticks.map((v) => ({ v, y: Y(v) })),
    xTicks,
    todayX: X(todayIso),
    zeroY: lo < 0 ? Y(0) : null,
    pad, width, height,
    empty: false
  }
}

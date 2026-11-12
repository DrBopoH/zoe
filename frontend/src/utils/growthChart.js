// График роста вложения для вкладки «Рынок»: чистая математика и координаты, без Vue.
// Одна линия = одна бумага. Ось X — годы от сегодня (отрицательные — прошлое), ось Y — сумма вложения в валюте бумаги.
// Бумага держит свою ставку до погашения (сложный процент); если до конца горизонта остаётся время, дальше пунктир: деньги
// лежат под базовой ставкой группы. «Сгоревшие» (уже погашенные) бумаги — серым пунктиром с даты выпуска до погашения.
import { HORIZON_YEARS } from './marketTable.js'
import { zoomWindow, panWindow, panY, zoomY, logTicks, scaleOf } from './chartView.js'

export const YEAR_DAYS = 365.25
export const MIN_SPAN = 0.25 // дальше приближать смысла нет: четверть года
export const MAX_SPAN = 60
const MAX_FUTURE = 30 // дальше 30 лет от сегодня линии не строим
export const GROWTH_PAD = { l: 92, r: 22, t: 14, b: 38 } // поля графика (px во viewBox): по ним мышь переводится в данные
const DEFAULT_RIGHT = 15 // окно по умолчанию не шире 15 лет вперёд: самая долгая бумага иначе сплющит остальные, дальше — колесом

// Во сколько раз вырастет вложение к моменту t (лет от сегодня)
export function multipleAt(sc, t, reinvest) {
  const held = Math.min(t, sc.years)
  let m = Math.pow(1 + sc.rate / 100, held)
  if (reinvest && t > sc.years) m *= Math.pow(1 + sc.base / 100, t - sc.years)
  return m
}

const decimalsOf = (step) => Math.max(0, -Math.floor(Math.log10(step) + 1e-9))

// Круглые деления шкалы: 1, 2, 5 × 10^n, не больше max штук. Значения округлены без хвоста плавающей точки (1.4, а не 1.4 с хвостом из нулей и единицы)
export function niceTicks(lo, hi, max = 6) {
  const span = hi - lo
  if (!(span > 0)) return [lo]
  const raw = span / max
  const pow = Math.pow(10, Math.floor(Math.log10(raw)))
  const step = [1, 2, 5, 10].map((k) => k * pow).find((s) => s >= raw)
  const d = decimalsOf(step)
  const out = []
  for (let v = Math.ceil(lo / step - 1e-9) * step; v <= hi + step * 1e-9; v += step) out.push(Number(v.toFixed(d)))
  return out
}

// Деления оси значения для любой шкалы: lo, hi — границы в шкальном пространстве (при log — логарифмы). Возвращает значения.
export function scaleTicks(mode, lo, hi, max = 6) {
  if (mode !== 'log') return niceTicks(lo, hi, max)
  const t = logTicks(lo, hi, max + 1)
  return t.length ? t : niceTicks(scaleOf('log').inv(lo), scaleOf('log').inv(hi), max)
}

// Первое число месяца через n месяцев от даты (UTC)
const addMonths = (d, n) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + n, 1))

// Деления оси времени: годы, если окно широкое, иначе месяцы. Возвращает [{ t, label }] (t — годы от today)
export function timeTicks(x0, x1, today) {
  const t0 = Date.parse(`${today}T00:00:00Z`)
  const dateAt = (t) => new Date(t0 + t * YEAR_DAYS * 86400000)
  const tOf = (d) => (d.getTime() - t0) / (YEAR_DAYS * 86400000)
  const span = x1 - x0
  const out = []
  if (!isFinite(x0) || !isFinite(x1) || !(span > 0)) return out // NaN в окне не должен зациклить деления
  if (span >= 3) {
    const step = span > 40 ? 10 : span > 20 ? 5 : span > 9 ? 2 : 1
    const y0 = dateAt(x0).getUTCFullYear()
    for (let y = Math.ceil(y0 / step) * step, guard = 0; guard < 200; y += step, guard++) {
      const t = tOf(new Date(Date.UTC(y, 0, 1)))
      if (t > x1) break
      if (t >= x0) out.push({ t, label: String(y) })
    }
  } else {
    const step = span > 1.5 ? 6 : span > 0.7 ? 3 : 1
    const d0 = dateAt(x0)
    for (let d = addMonths(d0, 1), guard = 0; guard < 200; d = addMonths(d, step), guard++) {
      const t = tOf(d)
      if (t > x1) break
      if (t >= x0) out.push({ t, label: `${String(d.getUTCMonth() + 1).padStart(2, '0')}.${d.getUTCFullYear()}` })
    }
  }
  return out
}

/**
 * Окно по умолчанию: от сегодня до самой долгой из лучших бумаг (но не меньше горизонта и не дальше 15 лет); если есть
 * сгоревшие — слева до самой ранней даты выпуска.
 */
export function autoView(items, matured = []) {
  const tops = items.filter((i) => i.top)
  const basis = tops.length ? tops : items
  const right = Math.min(DEFAULT_RIGHT, Math.max(HORIZON_YEARS, ...basis.map((i) => i.sc.years)))
  const left = matured.length ? Math.min(0, ...matured.map((m) => m.startT)) : 0
  return { x0: left, x1: right + (right - left) * 0.02 }
}

/** Окно после приближения: factor < 1 — приблизить, > 1 — отдалить; якорь anchor (лет) остаётся на месте. Общая математика — utils/chartView.js. */
export const zoomView = (view, factor, anchor) => zoomWindow(view, factor, anchor, { minSpan: MIN_SPAN, maxSpan: MAX_SPAN })

/** Сдвиг окна на dt лет. */
export const panView = panWindow

const MAX_PAST = 40 // бумаги старше 40 лет не рисуем: окно не должно уезжать в прошлый век из-за одной кривой даты

/** Годы от сегодня до даты выпуска/погашения сгоревших, без мусора: нечисловое и слишком давнее отбрасывается. */
export const cleanMatured = (list) => (Array.isArray(list) ? list : []).filter((m) => isFinite(m.startT) && isFinite(m.endT) && isFinite(m.rate) && m.startT >= -MAX_PAST && m.endT > m.startT)

/**
 * Диапазоны суммы по вертикали для окна view: auto — подгонка под лучшие бумаги в окне (+ сгоревшие, если есть);
 * ext — крайности по всем данным (лучшие бумаги на всём их протяжении, вложение, сгоревшие): за них ручной сдвиг не уходит.
 */
export function yRanges({ items, matured = [], amount = 1, view }) {
  const list = Array.isArray(items) ? items : []
  const old = cleanMatured(matured)
  const { x0, x1 } = view
  const tops = list.filter((i) => i.top)
  const basis = tops.length ? tops : list
  const val = (i, t) => amount * multipleAt(i.sc, t, true)
  const grow = (m, t) => amount * Math.pow(1 + m.rate / 100, t - m.startT)
  const range = (vals) => {
    let lo = Math.min(...vals, Infinity), hi = Math.max(...vals, -Infinity)
    if (!isFinite(lo) || !isFinite(hi)) { lo = amount; hi = amount * 1.05 }
    if (!(hi > lo)) hi = lo * 1.05
    const pad = (hi - lo) * 0.06
    return { lo: lo - pad, hi: hi + pad }
  }
  const clamp = (t) => Math.min(Math.max(t, x0), x1)
  const inWin = []
  for (const i of basis) inWin.push(val(i, Math.max(0, clamp(0))), val(i, clamp(Math.min(x1, MAX_FUTURE))))
  for (const m of old) {
    const a = Math.max(m.startT, x0), b = Math.min(m.endT, x1)
    if (a <= b) inWin.push(grow(m, a), grow(m, b))
  }
  if (x0 < 0 || old.length) inWin.push(amount)
  const all = [amount]
  for (const i of basis) all.push(val(i, Math.min(i.sc.years, MAX_FUTURE)), val(i, MAX_FUTURE))
  for (const m of old) all.push(grow(m, m.endT))
  return { auto: range(inWin), ext: range(all) }
}

/**
 * items: [{ id, sc, color, top }] — активные бумаги (sc из scoreBonds). matured: [{ id, rate, startT, endT }] — сгоревшие
 * (годы от сегодня, отрицательные). view: { x0, x1 } или null (авто). yRange: { lo, hi } или null (авто, под лучшие бумаги в окне).
 * Остальные линии могут уходить за край — их обрезает рамка графика.
 */
export function buildGrowthChart({ items, matured = [], amount = 1, view = null, yRange = null, today, width = 1000, height = 300, pad = GROWTH_PAD }) {
  const list = Array.isArray(items) ? items : []
  const old = cleanMatured(matured)
  const v = view || autoView(list, old)
  const { x0, x1 } = v
  const W = width - pad.l - pad.r
  const H = height - pad.t - pad.b
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * W
  const valueAt = (item, t) => amount * multipleAt(item.sc, t, true)

  const yr = yRanges({ items: list, matured: old, amount, view: v })
  const { lo, hi } = yRange || yr.auto
  const Y = (val) => pad.t + (1 - (val - lo) / (hi - lo)) * H

  // Кривая по видимой части отрезка [a, b]: точек столько, чтобы хватало на ~3px (при сильном приближении ломаная не видна)
  const curve = (a, b, f) => {
    const from = Math.max(a, x0 - (x1 - x0) * 0.02), to = Math.min(b, x1 + (x1 - x0) * 0.02)
    if (!(to > from)) return ''
    const n = Math.min(240, Math.max(12, Math.ceil(((to - from) / (x1 - x0)) * (W / 3))))
    const pts = []
    for (let s = 0; s <= n; s++) { const t = from + ((to - from) * s) / n; pts.push(`${X(t).toFixed(1)},${Y(f(t)).toFixed(1)}`) }
    return 'M' + pts.join('L')
  }

  const lines = list.map((item) => {
    const T = item.sc.years
    const endT = Math.min(T, MAX_FUTURE)
    return {
      id: item.id, color: item.color, top: !!item.top,
      solid: curve(0, endT, (t) => valueAt(item, t)),
      dashed: T < HORIZON_YEARS ? curve(T, HORIZON_YEARS, (t) => valueAt(item, t)) : '',
      mat: T <= MAX_FUTURE ? { x: X(T), y: Y(valueAt(item, T)), t: T, v: valueAt(item, T) } : null,
      end: { x: X(endT), y: Y(valueAt(item, endT)), v: valueAt(item, endT), t: endT }
    }
  })

  const burned = old.map((m) => {
    const grow = (t) => amount * Math.pow(1 + m.rate / 100, t - m.startT)
    return { id: m.id, d: curve(m.startT, m.endT, grow), end: { x: X(m.endT), y: Y(grow(m.endT)), v: grow(m.endT) } }
  })

  const xTicks = timeTicks(x0, x1, today).map((t) => ({ x: X(t.t), label: t.label }))
  const yTicks = niceTicks(lo, hi, 6).map((val) => ({ y: Y(val), v: val }))
  return {
    lines, burned, xTicks, yTicks, view: v, lo, hi, ranges: yr, pad, width, height, plotH: H,
    nowX: X(0), horizonX: X(HORIZON_YEARS),
    // обратные преобразования для мыши: пиксель (в координатах viewBox) → годы / сумма
    tAt: (px) => x0 + ((px - pad.l) / W) * (x1 - x0),
    vAt: (py) => lo + (1 - (py - pad.t) / H) * (hi - lo),
    valueOf: (id, t) => { const it = list.find((i) => i.id === id); return it ? valueAt(it, Math.max(0, t)) : null }
  }
}

// ручной сдвиг и масштаб суммы по вертикали — общие (utils/chartView.js), здесь реэкспорт для старых импортов
export { panY, zoomY }

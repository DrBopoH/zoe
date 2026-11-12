// График роста вложения для вкладки «Рынок»: чистая математика и координаты, без Vue.
// Одна линия = одна бумага. Ось X — годы от сегодня (отрицательные — прошлое), ось Y — сумма вложения в валюте бумаги.
// Бумага держит свою ставку до погашения (сложный процент); если до конца горизонта остаётся время, дальше пунктир: деньги
// лежат под базовой ставкой группы. «Сгоревшие» (уже погашенные) бумаги — серым пунктиром с даты выпуска до погашения.
import { HORIZON_YEARS } from './marketTable.js'

export const YEAR_DAYS = 365.25
export const MIN_SPAN = 0.25 // дальше приближать смысла нет: четверть года
export const MAX_SPAN = 60
const MAX_FUTURE = 30 // дальше 30 лет от сегодня линии не строим
const DEFAULT_RIGHT = 15 // окно по умолчанию не шире 15 лет вперёд: самая долгая бумага иначе сплющит остальные, дальше — колесом

// Во сколько раз вырастет вложение к моменту t (лет от сегодня)
export function multipleAt(sc, t, reinvest) {
  const held = Math.min(t, sc.years)
  let m = Math.pow(1 + sc.rate / 100, held)
  if (reinvest && t > sc.years) m *= Math.pow(1 + sc.base / 100, t - sc.years)
  return m
}

const decimalsOf = (step) => Math.max(0, -Math.floor(Math.log10(step) + 1e-9))

// Круглые деления шкалы: 1, 2, 5 × 10^n, не больше max штук. Значения округлены без хвоста плавающей точки (1.4, а не 1.4000000000000001)
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

// Первое число месяца через n месяцев от даты (UTC)
const addMonths = (d, n) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + n, 1))

// Деления оси времени: годы, если окно широкое, иначе месяцы. Возвращает [{ t, label }] (t — годы от today)
export function timeTicks(x0, x1, today) {
  const t0 = Date.parse(`${today}T00:00:00Z`)
  const dateAt = (t) => new Date(t0 + t * YEAR_DAYS * 86400000)
  const tOf = (d) => (d.getTime() - t0) / (YEAR_DAYS * 86400000)
  const span = x1 - x0
  const out = []
  if (span >= 3) {
    const step = span > 40 ? 10 : span > 20 ? 5 : span > 9 ? 2 : 1
    const y0 = dateAt(x0).getUTCFullYear()
    for (let y = Math.ceil(y0 / step) * step; ; y += step) {
      const t = tOf(new Date(Date.UTC(y, 0, 1)))
      if (t > x1) break
      if (t >= x0) out.push({ t, label: String(y) })
    }
  } else {
    const step = span > 1.5 ? 6 : span > 0.7 ? 3 : 1
    const d0 = dateAt(x0)
    for (let d = addMonths(d0, 1); ; d = addMonths(d, step)) {
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

/** Окно после приближения: factor < 1 — приблизить, > 1 — отдалить; якорь anchor (лет) остаётся на месте. */
export function zoomView(view, factor, anchor) {
  let span = (view.x1 - view.x0) * factor
  span = Math.min(MAX_SPAN, Math.max(MIN_SPAN, span))
  const ratio = (anchor - view.x0) / (view.x1 - view.x0)
  const x0 = anchor - ratio * span
  return { x0, x1: x0 + span }
}

/** Сдвиг окна на dt лет. */
export const panView = (view, dt) => ({ x0: view.x0 + dt, x1: view.x1 + dt })

/**
 * items: [{ id, sc, color, top }] — активные бумаги (sc из scoreBonds). matured: [{ id, rate, startT, endT }] — сгоревшие
 * (годы от сегодня, отрицательные). view: { x0, x1 } или null (авто). Масштаб по вертикали подгоняется под лучшие бумаги в окне:
 * остальные линии могут уходить за край — их обрезает рамка графика.
 */
export function buildGrowthChart({ items, matured = [], amount = 1, view = null, today, width = 1000, height = 300, pad = { l: 92, r: 22, t: 14, b: 26 } }) {
  const list = Array.isArray(items) ? items : []
  const old = Array.isArray(matured) ? matured : []
  const v = view || autoView(list, old)
  const { x0, x1 } = v
  const W = width - pad.l - pad.r
  const H = height - pad.t - pad.b
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * W

  const valueAt = (item, t) => amount * multipleAt(item.sc, t, true)
  const tops = list.filter((i) => i.top)
  const basis = tops.length ? tops : list
  const vis = (t) => Math.min(Math.max(t, x0), x1)
  // Диапазон суммы: по лучшим бумагам в окне (+ сгоревшие, если показаны) — от меньшего значения у левого края до большего у правого
  const samples = []
  for (const i of basis) { samples.push(valueAt(i, Math.max(0, vis(0)))); samples.push(valueAt(i, vis(x1))) }
  for (const m of old) {
    const a = Math.max(m.startT, x0), b = Math.min(m.endT, x1)
    if (a > b) continue
    samples.push(amount * Math.pow(1 + m.rate / 100, a - m.startT), amount * Math.pow(1 + m.rate / 100, b - m.startT))
  }
  if (x0 < 0 || old.length) samples.push(amount)
  let lo = Math.min(...samples, amount * 1e9)
  let hi = Math.max(...samples, 0)
  if (!isFinite(lo) || lo >= amount * 1e9) lo = amount
  if (!(hi > lo)) hi = lo * 1.05
  const padY = (hi - lo) * 0.06
  lo -= padY; hi += padY
  const Y = (val) => pad.t + (1 - (val - lo) / (hi - lo)) * H

  const path = (pts) => 'M' + pts.map(([t, val]) => `${X(t).toFixed(1)},${Y(val).toFixed(1)}`).join('L')
  const sample = (a, b, f, n = 40) => Array.from({ length: n + 1 }, (_, s) => { const t = a + ((b - a) * s) / n; return [t, f(t)] })

  const lines = list.map((item) => {
    const T = item.sc.years
    const solid = path(sample(0, Math.min(T, MAX_FUTURE), (t) => valueAt(item, t)))
    const dashed = T < HORIZON_YEARS ? path(sample(T, HORIZON_YEARS, (t) => valueAt(item, t), 24)) : ''
    const endT = Math.min(T, MAX_FUTURE)
    return {
      id: item.id, color: item.color, top: !!item.top,
      solid, dashed,
      mat: T <= MAX_FUTURE ? { x: X(T), y: Y(valueAt(item, T)), t: T, v: valueAt(item, T) } : null,
      end: { x: X(endT), y: Y(valueAt(item, endT)), v: valueAt(item, endT), t: endT }
    }
  })

  const burned = old.map((m) => ({
    id: m.id,
    d: path(sample(m.startT, m.endT, (t) => amount * Math.pow(1 + m.rate / 100, t - m.startT), 24)),
    end: { x: X(m.endT), y: Y(amount * Math.pow(1 + m.rate / 100, m.endT - m.startT)), v: amount * Math.pow(1 + m.rate / 100, m.endT - m.startT) }
  }))

  const xTicks = timeTicks(x0, x1, today).map((t) => ({ x: X(t.t), label: t.label }))
  const yTicks = niceTicks(lo, hi, 6).map((val) => ({ y: Y(val), v: val }))
  return {
    lines, burned, xTicks, yTicks, view: v, lo, hi, pad, width, height,
    nowX: X(0), horizonX: X(HORIZON_YEARS),
    // обратное преобразование для мыши: пиксель (в координатах viewBox) → годы
    tAt: (px) => x0 + ((px - pad.l) / W) * (x1 - x0),
    // сумма лучшей бумаги на момент t (для подсказки под курсором)
    valueOf: (id, t) => { const it = list.find((i) => i.id === id); return it ? valueAt(it, Math.max(0, t)) : null }
  }
}

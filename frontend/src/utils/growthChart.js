// Графики роста вложения для вкладки «Рынок»: чистая математика и координаты, без Vue.
// Одна линия = одна бумага. Бумага держит свою ставку до погашения (сложный процент), потом (в «сравнимом» режиме) остаток
// горизонта деньги лежат под базовой ставкой группы; в «реальном» режиме линия просто кончается в день погашения.
import { HORIZON_YEARS } from './marketTable.js'

// Во сколько раз вырастет вложение к моменту t (лет от сегодня)
export function multipleAt(sc, t, reinvest) {
  const held = Math.min(t, sc.years)
  let m = Math.pow(1 + sc.rate / 100, held)
  if (reinvest && t > sc.years) m *= Math.pow(1 + sc.base / 100, t - sc.years)
  return m
}

// Круглые деления шкалы: 1, 2, 5 × 10^n, не больше max штук
export function niceTicks(lo, hi, max = 6) {
  const span = hi - lo
  if (!(span > 0)) return [lo]
  const raw = span / max
  const pow = Math.pow(10, Math.floor(Math.log10(raw)))
  const step = [1, 2, 5, 10].map((k) => k * pow).find((s) => s >= raw)
  const out = []
  for (let v = Math.ceil(lo / step) * step; v <= hi + step * 1e-9; v += step) out.push(Math.round(v / step) * step)
  return out
}

const MAX_YEARS = 30 // дальше 30 лет «реальный» график не растягиваем: самая долгая бумага уйдёт за край

/**
 * items: [{ id, sc, color, top }] — sc из scoreBonds. mode: 'relative' (сравнимый, 0…HORIZON_YEARS, по вертикали «×») или
 * 'absolute' (реальный: по горизонтали годы до погашения каждой бумаги, по вертикали сумма amount × кратность).
 * Масштаб по вертикали задают лучшие бумаги (top): остальные линии могут уходить за край — их обрезает рамка графика.
 */
export function buildGrowthChart({ items, mode, amount = 1, width = 1000, height = 220, pad = { l: 92, r: 22, t: 14, b: 24 }, todayYear = new Date().getFullYear() }) {
  const rel = mode === 'relative'
  const list = Array.isArray(items) ? items : []
  const tops = list.filter((i) => i.top)
  const basis = tops.length ? tops : list
  const tMax = rel ? HORIZON_YEARS : Math.min(MAX_YEARS, Math.max(1, ...basis.map((i) => i.sc.years)))
  const k = rel ? 1 : amount
  const lo = k // старт: вложение = 1× (или amount)
  const valueTo = (i) => k * multipleAt(i.sc, Math.min(rel ? tMax : i.sc.years, tMax), rel)
  const hi = Math.max(lo * 1.05, ...basis.map(valueTo)) * 1.06
  const W = width - pad.l - pad.r
  const H = height - pad.t - pad.b
  const X = (t) => pad.l + (t / tMax) * W
  const Y = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * H
  const pt = (t, reinvest) => `${X(t).toFixed(1)},${Y(k * multipleAt(i0, t, reinvest)).toFixed(1)}`
  let i0 = null

  const lines = list.map((item) => {
    i0 = item.sc
    const T = item.sc.years
    const steps = 48
    const upto = Math.min(T, tMax)
    const solid = []
    for (let s = 0; s <= steps; s++) solid.push(pt((upto * s) / steps, rel))
    let dashed = ''
    if (rel && T < tMax) {
      const part = []
      for (let s = 0; s <= 24; s++) part.push(pt(T + ((tMax - T) * s) / 24, true))
      dashed = 'M' + part.join('L')
    }
    const endT = upto
    const endV = k * multipleAt(item.sc, endT, rel)
    return {
      id: item.id, color: item.color, top: !!item.top,
      solid: 'M' + solid.join('L'), dashed,
      end: { x: X(endT), y: Y(endV), v: endV, t: endT, matured: T <= tMax },
      // точка погашения на линии (в сравнимом режиме после неё идёт пунктир); за краем графика её нет
      mat: T <= tMax ? { x: X(T), y: Y(k * multipleAt(item.sc, T, rel)) } : null
    }
  })

  const xTicks = niceTicks(0, tMax, 6).map((t) => ({ x: X(t), label: rel ? `${t} л.` : String(todayYear + t) }))
  const yTicks = niceTicks(lo, hi, 5).map((v) => ({ y: Y(v), v }))
  return { lines, xTicks, yTicks, tMax, lo, hi, pad, width, height }
}

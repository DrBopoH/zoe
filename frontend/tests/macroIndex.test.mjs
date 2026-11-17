import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildMacroChart, macroRanges, thinColumns, yearsFrom, isoAt, SERIES } from '../src/utils/macro.js'
import { robustRanges, scaleOf } from '../src/utils/chartView.js'

// Эталон — прежний перебор всех точек с Date.parse: индекс и бисекция обязаны дать тот же результат бит в бит.
const TODAY = '2026-11-17'
let seed = 7
const rnd = () => ((seed = (seed * 1664525 + 1013904223) >>> 0) / 2 ** 32)
const days = (from, n) => { const out = []; let t = Date.parse(`${from}T00:00:00Z`); for (let i = 0; i < n; i++, t += 86400000) out.push(new Date(t).toISOString().slice(0, 10)); return out }
const walk = (n, base, from = '2005-03-01') => { let v = base; return days(from, n).map((d) => ({ d, v: (v *= 1 + (rnd() - 0.5) * 0.02) })) }
const withZeros = (pts) => pts.map((p, i) => (i % 97 === 0 ? { d: p.d, v: i % 194 === 0 ? 0 : -1.5 } : p))

function naiveVisible(pts, view) {
  const out = []
  for (const p of pts) { const t = yearsFrom(p.d, TODAY); if (t >= view.x0 && t <= view.x1) out.push(p.v) }
  return out
}
function naivePath(pts, view, { step, log }) {
  const sc = scaleOf(log ? 'log' : 'linear'), margin = (view.x1 - view.x0) * 0.05
  const out = []
  let prev = null
  for (const p of pts) {
    if (sc.mode === 'log' && !(p.v > 0)) continue
    const t = yearsFrom(p.d, TODAY)
    if (t < view.x0 - margin) { prev = { t: view.x0 - margin, v: p.v }; continue }
    if (t > view.x1 + margin) break
    if (prev) { out.push(prev); prev = null }
    out.push({ t, v: p.v })
  }
  if (prev) out.push(prev)
  return out
}

const VIEWS = [{ x0: -30, x1: 0.6 }, { x0: -5, x1: 0.05 }, { x0: -1, x1: 0.05 }, { x0: -0.5, x1: 0 }, { x0: -40, x1: 1 }, { x0: -2.37, x1: -1.1 }, { x0: 3, x1: 5 }]

test('macroRanges: окно и крайности из индекса совпадают с перебором', () => {
  const series = { a: walk(9000, 20), b: withZeros(walk(3000, 5, '2015-01-01')), c: walk(1, 3, '2026-01-01') }
  for (const log of [false, true]) for (const view of VIEWS) for (const ids of [['a'], ['a', 'b'], ['b', 'c']]) {
    const got = macroRanges({ series, ids, view, todayIso: TODAY, log })
    const visible = [], all = []
    for (const id of ids) {
      const pts = series[id]
      const t0 = isoAt(view.x0, TODAY)
      let before = null
      for (const p of pts) if (p.d <= t0) before = p.v
      if (before !== null && pts[0].d <= t0) visible.push(before)
      for (const p of pts) all.push(p.v)
      visible.push(...naiveVisible(pts, view))
    }
    assert.deepEqual(got, robustRanges({ visible, all, floor: 0, log }), `log=${log} view=${JSON.stringify(view)} ids=${ids}`)
  }
})

test('buildMacroChart: точки окна и вход слева совпадают с перебором', () => {
  const series = { policy_rate: walk(4000, 10), usd_uah: walk(8000, 20), z: withZeros(walk(2500, 4, '2012-01-01')) }
  for (const scale of ['linear', 'log']) for (const view of VIEWS) for (const id of ['policy_rate', 'usd_uah']) {
    const chart = buildMacroChart({ series, ids: [id], view, yRange: { lo: scale === 'log' ? 0 : 0, hi: 100 }, todayIso: TODAY, scale })
    const sc = scaleOf(scale), pad = chart.pad
    const X = (t) => pad.l + ((t - view.x0) / (view.x1 - view.x0)) * (chart.width - pad.l - pad.r)
    const Y = (v) => pad.t + (1 - (sc.fwd(v) - 0) / (100 - 0)) * (chart.height - pad.t - pad.b)
    const meta = SERIES[id] || { step: false }
    let d = ''
    const naive = naivePath(series[id], view, { log: scale === 'log' })
    ;(meta.step ? naive : thinColumns(naive, X, 2 * (chart.width - pad.l - pad.r))).forEach((p, i) => {
      const x = X(p.t).toFixed(1), y = Y(p.v).toFixed(1)
      d += i === 0 ? `M${x} ${y}` : meta.step ? `H${x}V${y}` : `L${x} ${y}`
    })
    const margin = (view.x1 - view.x0) * 0.05
    if (meta.step && d) d += `H${X(Math.min(view.x1 + margin, 0)).toFixed(1)}`
    assert.equal(chart.lines[0].d, d, `${scale} ${id} ${JSON.stringify(view)}`)
  }
})

test('buildMacroChart: логарифм пропускает неположительные точки перед окном', () => {
  const series = { usd_uah: withZeros(walk(1200, 20, '2022-01-01')) }
  const view = { x0: -1, x1: 0.05 }
  const chart = buildMacroChart({ series, ids: ['usd_uah'], view, yRange: { lo: 0, hi: 10 }, todayIso: TODAY, scale: 'log' })
  const ref = naivePath(series.usd_uah, view, { log: true })
  assert.equal(chart.lines[0].d.split('L').length, thinColumns(ref, () => 0, Infinity).length)
})

test('индекс обновляется, если ряд дополнили на месте', () => {
  const pts = walk(500, 10)
  const view = { x0: -3, x1: 0.05 }
  const before = macroRanges({ series: { a: pts }, ids: ['a'], view, todayIso: TODAY })
  pts.push({ d: '2026-11-17', v: 1e6 })
  const after = macroRanges({ series: { a: pts }, ids: ['a'], view, todayIso: TODAY })
  assert.ok(after.ext.hi > before.ext.hi * 100)
})

test('thinColumns: короткий ряд не меняется, плотный сводится к ≤4 точкам на столбец и хранит крайности', () => {
  const X = (t) => t * 100
  const few = Array.from({ length: 50 }, (_, i) => ({ t: i / 50, v: i }))
  assert.equal(thinColumns(few, X, 100), few)
  const dense = Array.from({ length: 20000 }, (_, i) => ({ t: i / 20000, v: Math.sin(i / 37) * 10 + rnd() }))
  const out = thinColumns(dense, X, 400)
  assert.ok(out.length <= 4 * 100, `points ${out.length}`)
  assert.equal(out[0], dense[0])
  assert.equal(out[out.length - 1], dense[dense.length - 1])
  assert.equal(Math.max(...out.map((p) => p.v)), Math.max(...dense.map((p) => p.v)))
  assert.equal(Math.min(...out.map((p) => p.v)), Math.min(...dense.map((p) => p.v)))
  assert.ok(out.every((p, i) => i === 0 || p.t >= out[i - 1].t), 'порядок времени сохранён')
  // каждый столбец: минимум и максимум столбца целы
  const cols = new Map()
  for (const p of dense) { const c = Math.floor(X(p.t)); const e = cols.get(c) || [Infinity, -Infinity]; cols.set(c, [Math.min(e[0], p.v), Math.max(e[1], p.v)]) }
  const kept = new Map()
  for (const p of out) { const c = Math.floor(X(p.t)); const e = kept.get(c) || [Infinity, -Infinity]; kept.set(c, [Math.min(e[0], p.v), Math.max(e[1], p.v)]) }
  assert.deepEqual([...kept], [...cols])
})

test('robustRanges на типизированной сортировке совпадает с обычной', () => {
  const vals = Array.from({ length: 5000 }, () => (rnd() - 0.4) * 50)
  vals.push(NaN, Infinity, null, 'x')
  const got = robustRanges({ visible: vals, all: vals })
  const sorted = vals.filter((v) => typeof v === 'number' && isFinite(v)).sort((a, b) => a - b)
  const q = (p) => { const i = (sorted.length - 1) * p, a = Math.floor(i), b = Math.ceil(i); return sorted[a] + (sorted[b] - sorted[a]) * (i - a) }
  const make = (lo, hi) => { const pd = (hi - lo) * 0.06; return { lo: lo - pd, hi: hi + pd } }
  const ext = make(sorted[0], sorted[sorted.length - 1]), auto = make(q(0.02), q(0.98))
  assert.deepEqual(got.ext, ext)
  assert.deepEqual(got.auto, { lo: Math.max(auto.lo, ext.lo), hi: Math.min(auto.hi, ext.hi) })
})

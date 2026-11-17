import test from 'node:test'
import assert from 'node:assert/strict'
import { TFS, dayStartOf, weekStartOf, alignDown, buildIntradayStarts, buildDailyStarts, lowerBound, niceStep, axisLabel } from '../src/utils/candleMath.js'

test('dayStartOf / weekStartOf: полночь и понедельник', () => {
  const d = new Date(2026, 10, 12, 15, 30) // чт
  assert.equal(dayStartOf(d.getTime()), new Date(2026, 10, 12).getTime())
  assert.equal(weekStartOf(d.getTime()), new Date(2026, 10, 9).getTime())
  const sun = new Date(2026, 10, 15, 23, 0)
  assert.equal(weekStartOf(sun.getTime()), new Date(2026, 10, 9).getTime())
})

test('alignDown кратен шагу от начала дня', () => {
  const t = new Date(2026, 10, 12, 10, 47).getTime()
  assert.equal(alignDown(t, 900_000), new Date(2026, 10, 12, 10, 45).getTime())
})

test('lowerBound', () => {
  assert.equal(lowerBound([1, 3, 5], 0), 0)
  assert.equal(lowerBound([1, 3, 5], 3), 1)
  assert.equal(lowerBound([1, 3, 5], 4), 2)
  assert.equal(lowerBound([1, 3, 5], 9), 3)
})

test('niceStep', () => {
  assert.equal(niceStep(0.9), 1)
  assert.equal(niceStep(1.7), 2)
  assert.equal(niceStep(2.2), 2.5)
  assert.equal(niceStep(4), 5)
  assert.equal(niceStep(7), 10)
  assert.equal(niceStep(130), 200)
})

test('axisLabel: единицы к/М и знаки', () => {
  assert.equal(axisLabel(9500, 500, 9000, 11000), '9,5к')
  assert.equal(axisLabel(2_000_000, 500_000, 0, 3e6), '2,0М')
  assert.equal(axisLabel(250, 50, 0, 900), '250')
})

test('buildDailyStarts: шаг день/неделя, последний — текущий', () => {
  const now = Date.now()
  const min = now - 20 * 86_400_000
  const days = buildDailyStarts(TFS.find(t => t.id === '1D'), min)
  assert.equal(days.at(-1), dayStartOf(now))
  assert.ok(days.length >= 20 && days.length <= 22)
  const weeks = buildDailyStarts(TFS.find(t => t.id === '1W'), min)
  assert.equal(weeks.at(-1), weekStartOf(now))
})

test('buildIntradayStarts: возрастание, не больше want, обрезка по краю истории', () => {
  const s = buildIntradayStarts(3_600_000, 0, 100)
  assert.equal(s.length, 100)
  for (let i = 1; i < s.length; i++) assert.ok(s[i] > s[i - 1])
  assert.ok(s.at(-1) <= Date.now())
  const near = buildIntradayStarts(3_600_000, Date.now() - 5 * 3_600_000, 1000)
  assert.ok(near.length <= 7)
})

test('candleMove: изменение за период и размах внутри него, проценты от открытия', async () => {
  const { candleMove } = await import('../src/utils/candleMath.js')
  const m = candleMove({ open: 9000, close: 8600, high: 9100, low: 8540, bnOpen: 4000, bnClose: 3650.5 })
  assert.equal(m.delta, -400)
  assert.equal(m.deltaPct, -4.4)
  assert.equal(m.swing, 560)
  assert.equal(m.swingPct, 6.2)
  assert.equal(m.bnDelta, -349.5)
  assert.equal(candleMove({ open: 100, close: 100, high: 100, low: 100 }).delta, 0)
  assert.equal(Object.is(candleMove({ open: 100, close: 100, high: 100, low: 100 }).delta, -0), false)
})

test('candleMove: нет данных свечи или нулевое открытие', async () => {
  const { candleMove } = await import('../src/utils/candleMath.js')
  assert.equal(candleMove(null), null)
  assert.equal(candleMove({ key: 'x', from: 1, to: 2 }), null) // выбор без OHLC
  assert.equal(candleMove({ open: 1, close: NaN, high: 1, low: 1 }), null)
  const z = candleMove({ open: 0, close: 50, high: 60, low: 0 })
  assert.equal(z.delta, 50)
  assert.equal(z.deltaPct, null)
  assert.equal(z.bnDelta, null)
})

import { candleBounds, candleWindow, candleSpans, visibleSlots, candleYRanges, CANDLE_END_PAD } from '../src/utils/candleMath.js'
import { clampWindow, inertiaStep } from '../src/utils/chartView.js'

test('окно свечей: без будущего — у правого края, с будущим — «сегодня» на 70 %', () => {
  const w = candleWindow({ n: 100, future: 0, plotW: 880 })
  assert.equal(w.x1, CANDLE_END_PAD)
  assert.equal(w.x1 - w.x0, 40) // 880 / 22
  const f = candleWindow({ n: 100, future: 30, plotW: 880 })
  assert.ok(Math.abs(f.x1 - 12) < 1e-9)
})

test('границы и размах окна свечей', () => {
  assert.deepEqual(candleBounds(100, 5), { min: -101.4, max: 6.4 })
  const s = candleSpans({ n: 100, future: 0, plotW: 880 })
  assert.equal(s.minSpan, 8)
  assert.ok(Math.abs(s.maxSpan - 102.8) < 1e-9)
  assert.equal(candleSpans({ n: 5000, future: 0, plotW: 300 }).maxSpan, 200) // тоньше 1,5 px на свечу не уходим
})

test('clampWindow держит окно в границах и сохраняет размах', () => {
  const b = { min: -10, max: 5 }
  assert.deepEqual(clampWindow({ x0: -20, x1: -10 }, b), { x0: -10, x1: 0 })
  assert.deepEqual(clampWindow({ x0: 1, x1: 11 }, b), { x0: -5, x1: 5 })
  assert.deepEqual(clampWindow({ x0: -30, x1: 10 }, b), { x0: -10, x1: 30 }) // шире границ — к левой
  const v = { x0: -3, x1: 2 }
  assert.equal(clampWindow(v, b), v)
  assert.equal(clampWindow(v, null), v)
})

test('инерция затухает и заканчивается', () => {
  let v = 1, total = 0, steps = 0
  for (; steps < 1000; steps++) { const s = inertiaStep(v, 16); total += s.dx; v = s.v; if (s.done) break }
  assert.ok(steps > 10 && steps < 200)
  assert.ok(total > 0)
})

test('видимые слоты и вертикаль по свечам в окне', () => {
  assert.deepEqual(visibleSlots({ x0: -10, x1: 1.4 }, 100, 100), { a: 89, b: 100 })
  assert.deepEqual(visibleSlots({ x0: -500, x1: -490 }, 100, 100), { a: 0, b: 0 })
  const n = 10
  const L = Float64Array.from({ length: n }, (_, i) => i), H = Float64Array.from({ length: n }, (_, i) => i + 2)
  const S = { n, L, H, min: 0, max: 11 }
  const r = candleYRanges(S, { x0: -3, x1: 0 }) // слоты 7…9 (+1 запас слева)
  assert.ok(r.auto.lo < 7 && r.auto.lo > 5.5)
  assert.ok(r.auto.hi > 11 - 0.01 && r.ext.hi > 11)
  assert.deepEqual(candleYRanges(S, { x0: 5, x1: 9 }).auto, candleYRanges(S, { x0: 5, x1: 9 }).ext) // окно в будущем
})

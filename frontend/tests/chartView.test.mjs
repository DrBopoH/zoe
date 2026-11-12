import test from 'node:test'
import assert from 'node:assert/strict'
import { zoomWindow, panWindow, panY, zoomY, easeRange, quantile, robustRanges, scaleOf, logTicks } from '../src/utils/chartView.js'

test('zoomWindow: якорь стоит на месте, размах зажат пределами', () => {
  const z = zoomWindow({ x0: 0, x1: 10 }, 0.5, 4, { minSpan: 1, maxSpan: 20 })
  assert.equal(z.x1 - z.x0, 5)
  assert.ok(Math.abs((4 - z.x0) / 5 - 0.4) < 1e-9) // анкер остался на той же доле окна
  const span = (v) => v.x1 - v.x0
  assert.ok(Math.abs(span(zoomWindow({ x0: 0, x1: 10 }, 0.001, 4, { minSpan: 1 })) - 1) < 1e-9)
  assert.ok(Math.abs(span(zoomWindow({ x0: 0, x1: 10 }, 100, 4, { maxSpan: 20 })) - 20) < 1e-9)
  const free = zoomWindow({ x0: 0, x1: 10 }, 3, 5) // без пределов
  assert.ok(Math.abs(free.x1 - free.x0 - 30) < 1e-9)
})

test('panWindow и вертикаль: сдвиг в пределах крайностей, масштаб 2–120 %', () => {
  assert.deepEqual(panWindow({ x0: 1, x1: 3 }, 0.5), { x0: 1.5, x1: 3.5 })
  const ext = { lo: 0, hi: 100 }
  assert.deepEqual(panY({ lo: 10, hi: 30 }, -50, ext), { lo: 0, hi: 20 })
  assert.deepEqual(panY({ lo: 10, hi: 30 }, 500, ext), { lo: 80, hi: 100 })
  const z = zoomY({ lo: 0, hi: 100 }, 0.5, 25, ext)
  assert.equal(z.hi - z.lo, 50)
  assert.ok(zoomY({ lo: 0, hi: 1 }, 0.001, 0.5, ext).hi - zoomY({ lo: 0, hi: 1 }, 0.001, 0.5, ext).lo >= 2 - 1e-9)
})

test('easeRange: догоняет цель плавно и останавливается ровно на ней', () => {
  const target = { lo: 0, hi: 100 }
  let shown = { lo: 50, hi: 200 }
  let steps = 0
  for (;;) {
    const r = easeRange(shown, target)
    steps++
    assert.ok(steps < 200, 'не сошлось')
    if (r.done) { assert.deepEqual(r.range, target); break }
    assert.ok(Math.abs(r.range.hi - target.hi) < Math.abs(shown.hi - target.hi)) // приближается
    shown = r.range
  }
  assert.ok(steps > 3, 'шаг должен быть плавным, не прыжком')
  assert.deepEqual(easeRange(null, target), { range: target, done: true })
})

test('quantile: интерполяция по отсортированным', () => {
  assert.equal(quantile([1, 2, 3, 4, 5], 0), 1)
  assert.equal(quantile([1, 2, 3, 4, 5], 1), 5)
  assert.equal(quantile([1, 2, 3, 4, 5], 0.5), 3)
  assert.equal(quantile([0, 10], 0.25), 2.5)
  assert.ok(Number.isNaN(quantile([], 0.5)))
})

test('robustRanges: на малом числе точек выбросов не срезаем, на большом — срезаем хвост, ext — всё', () => {
  const few = robustRanges({ visible: [1, 2, 3, 100], all: [1, 2, 3, 100], pad: 0 })
  assert.equal(few.auto.hi, 100) // 4 точки: срезать нечего
  const many = Array.from({ length: 200 }, (_, i) => 5 + (i % 5))
  const withOutlier = [...many, -50]
  const r = robustRanges({ visible: withOutlier, all: withOutlier, pad: 0 })
  assert.ok(r.auto.lo >= 4.9, `auto.lo ${r.auto.lo}`)
  assert.equal(r.ext.lo, -50)
  assert.ok(r.auto.lo >= r.ext.lo && r.auto.hi <= r.ext.hi)
  const fl = robustRanges({ visible: [5, 6, 7], all: [5, 6, 7], pad: 0, floor: 0 })
  assert.equal(fl.auto.lo, 0) // нулевая линия видна
  const empty = robustRanges({ visible: [], all: [] })
  assert.ok(empty.auto.hi > empty.auto.lo)
})

test('scaleOf: linear — тождество, log — натуральный логарифм и обратно', () => {
  assert.equal(scaleOf('linear').fwd(5), 5)
  assert.equal(scaleOf('что-то').mode, 'linear')
  const L = scaleOf('log')
  assert.ok(Math.abs(L.inv(L.fwd(1234.5)) - 1234.5) < 1e-9)
  assert.ok(Math.abs(L.fwd(Math.E) - 1) < 1e-12)
})

test('robustRanges log: диапазон в логарифмах, неположительные значения отброшены, floor не действует', () => {
  const r = robustRanges({ visible: [10, 100, 1000, 0, -5], all: [10, 100, 1000, 0, -5], pad: 0, floor: 0, log: true })
  assert.ok(Math.abs(r.ext.lo - Math.log(10)) < 1e-9 && Math.abs(r.ext.hi - Math.log(1000)) < 1e-9)
  const empty = robustRanges({ visible: [], all: [], log: true })
  assert.ok(empty.ext.hi > empty.ext.lo)
})

test('logTicks: 1-3 / степени десяти по лимиту, 1-2-5 на узком, пусто на очень узком', () => {
  assert.deepEqual(logTicks(Math.log(5), Math.log(8000), 7), [10, 30, 100, 300, 1000, 3000])
  assert.deepEqual(logTicks(Math.log(5), Math.log(8000), 5), [10, 100, 1000]) // реже — только степени десяти
  assert.deepEqual(logTicks(Math.log(8), Math.log(120), 7), [10, 20, 50, 100])
  assert.deepEqual(logTicks(Math.log(100), Math.log(101), 7), [])
  assert.deepEqual(logTicks(1, 1), [])
})

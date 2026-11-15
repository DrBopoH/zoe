import test from 'node:test'
import assert from 'node:assert/strict'
import { SERIES, CURVE_TENORS, curveSnapshot, curveChart } from '../src/utils/macro.js'

const pts = (...vs) => vs.map(([d, v]) => ({ d, v }))
const series = {
  ust_3m: pts(['2021-11-10', 0.05], ['2025-11-10', 4.1], ['2026-11-12', 4.3]),
  ust_2y: pts(['2021-11-10', 0.5], ['2025-11-10', 3.9], ['2026-11-12', 4.0]),
  ust_10y: pts(['2025-11-10', 4.2], ['2026-11-12', 4.6])
}
const TODAY = '2026-11-12'

test('у каждого нового ряда есть подпись и цвет', () => {
  for (const id of [...CURVE_TENORS.map((t) => t.id).filter((id) => id !== 'ust_10y'), 'ust_10y', 'us_spread3m', 'us_real5', 'us_real10', 'us_be5', 'us_be10', 'us_fwd5y5y', 'us_termprem', 'us_mortgage']) {
    assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
  }
})

test('сроки кривой идут от короткого к длинному и включают 10 лет из источника us', () => {
  const ids = CURVE_TENORS.map((t) => t.id)
  assert.equal(ids[0], 'ust_1m')
  assert.equal(ids.at(-1), 'ust_30y')
  assert.ok(ids.includes('ust_10y'))
  assert.equal(new Set(ids).size, ids.length)
})

test('снимок «сейчас» берёт последние значения, «год назад» — значение на дату', () => {
  const now = curveSnapshot(series, TODAY, 0)
  assert.equal(now.find((p) => p.id === 'ust_3m').v, 4.3)
  assert.equal(now.find((p) => p.id === 'ust_10y').v, 4.6)
  assert.equal(now.find((p) => p.id === 'ust_30y').v, null, 'нет ряда — нет точки')
  const y1 = curveSnapshot(series, TODAY, 1)
  assert.equal(y1.find((p) => p.id === 'ust_3m').v, 4.1)
  assert.equal(y1.find((p) => p.id === 'ust_2y').v, 3.9)
  const y5 = curveSnapshot(series, TODAY, 5)
  assert.equal(y5.find((p) => p.id === 'ust_3m').v, 0.05)
  assert.equal(y5.find((p) => p.id === 'ust_10y').v, null, 'раньше первой точки ряда — пусто')
})

test('график кривой: пропуски рвут линию, пустые данные — empty', () => {
  const empty = curveChart([{ key: 'now', color: '#fff', points: curveSnapshot({}, TODAY, 0) }])
  assert.equal(empty.empty, true)
  const chart = curveChart([
    { key: 'now', color: '#38bdf8', points: curveSnapshot(series, TODAY, 0) },
    { key: 'y5', color: '#64748b', points: curveSnapshot(series, TODAY, 5) }
  ])
  assert.equal(chart.empty, false)
  assert.equal(chart.xs.length, CURVE_TENORS.length)
  assert.ok(chart.yTicks.length >= 2)
  const now = chart.lines.find((l) => l.key === 'now')
  // 3м, 2г и 10л есть, остальные пусты: три отдельных «M»
  assert.equal((now.d.match(/M/g) || []).length, 3)
  assert.ok(!now.d.includes('NaN'))
})

test('график с одним значением не падает: ось раздвигается', () => {
  const chart = curveChart([{ key: 'now', color: '#fff', points: curveSnapshot({ ust_2y: pts(['2026-11-12', 4]) }, TODAY, 0) }])
  assert.equal(chart.empty, false)
  assert.ok(chart.yTicks.length >= 1)
})

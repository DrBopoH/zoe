import test from 'node:test'
import assert from 'node:assert/strict'
import { multipleAt, niceTicks, buildGrowthChart } from '../src/utils/growthChart.js'

const sc = (years, rate, base = 10) => ({ years, rate, base })

test('кратность: до погашения под ставку бумаги, дальше (если режим сравнения) под базу', () => {
  assert.ok(Math.abs(multipleAt(sc(2, 10), 2, false) - 1.21) < 1e-9)
  assert.ok(Math.abs(multipleAt(sc(2, 10), 5, false) - 1.21) < 1e-9) // реальный режим: после погашения рост стоит
  assert.ok(Math.abs(multipleAt(sc(2, 20, 10), 4, true) - 1.44 * 1.21) < 1e-9)
  assert.equal(multipleAt(sc(5, 10), 0, true), 1)
})

test('круглые деления шкалы', () => {
  assert.deepEqual(niceTicks(0, 10, 5), [0, 2, 4, 6, 8, 10])
  assert.deepEqual(niceTicks(1, 4.5, 4), [1, 2, 3, 4])
  assert.deepEqual(niceTicks(5, 5), [5])
})

const items = [
  { id: 'L', color: 'red', top: false, sc: sc(2, 5) },
  { id: 'A', color: 'green', top: true, sc: sc(8, 16) },
  { id: 'B', color: 'lime', top: true, sc: sc(15, 14) }
]

test('сравнимый график: все линии идут до горизонта, после погашения пунктир', () => {
  const c = buildGrowthChart({ items, mode: 'relative', todayYear: 2026 })
  assert.equal(c.tMax, 10)
  const a = c.lines.find((l) => l.id === 'A'), b = c.lines.find((l) => l.id === 'B')
  assert.notEqual(a.dashed, ''); assert.equal(b.dashed, '') // у B погашение за горизонтом
  assert.equal(a.end.matured, true); assert.equal(b.end.matured, false)
  assert.equal(c.xTicks[0].label, '0 л.')
})

test('реальный график: линия кончается в день погашения, масштаб по лучшим, подписи — календарные годы', () => {
  const c = buildGrowthChart({ items, mode: 'absolute', amount: 1000, todayYear: 2026 })
  assert.equal(c.tMax, 15) // самая долгая из лучших
  const a = c.lines.find((l) => l.id === 'A')
  assert.equal(a.dashed, '')
  assert.ok(Math.abs(a.end.v - 1000 * Math.pow(1.16, 8)) < 1e-6)
  assert.ok(a.end.x < c.width - c.pad.r) // 8 лет из 15 — левее правого края
  const b = c.lines.find((l) => l.id === 'B')
  assert.ok(Math.abs(b.end.x - (c.width - c.pad.r)) < 1e-6) // самая долгая упирается в правый край
  assert.equal(c.xTicks[0].label, '2026')
  const low = c.lines.find((l) => l.id === 'L')
  assert.ok(low.end.y > a.end.y) // меньше сумма — ниже на экране
})

test('пустой список и бумага дольше 30 лет не ломают построение', () => {
  assert.equal(buildGrowthChart({ items: [], mode: 'absolute' }).lines.length, 0)
  const c = buildGrowthChart({ items: [{ id: 'X', top: true, color: 'red', sc: sc(40, 10) }], mode: 'absolute', amount: 100 })
  assert.equal(c.tMax, 30)
  assert.equal(c.lines[0].end.matured, false)
})

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

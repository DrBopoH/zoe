import test from 'node:test'
import assert from 'node:assert/strict'
import { valueAt, realRate, signedPct, plainPct, currentInflation, macroSummary, buildMacroChart } from '../src/utils/macro.js'
import { sortBonds } from '../src/utils/marketTable.js'

const rate = [{ d: '2024-12-12', v: 13.5 }, { d: '2025-01-23', v: 14.5 }, { d: '2026-01-01', v: 15.5 }]
const cpi = [{ d: '2026-07-01', v: 5.6 }, { d: '2026-08-01', v: 5.9 }, { d: '2026-09-01', v: 6.1 }]

test('значение ставки на дату — последняя точка не позже даты, раньше первой — null', () => {
  assert.equal(valueAt(rate, '2024-12-11'), null)
  assert.equal(valueAt(rate, '2024-12-12'), 13.5)
  assert.equal(valueAt(rate, '2025-06-01'), 14.5)
  assert.equal(valueAt(rate, '2030-01-01'), 15.5)
  assert.equal(valueAt([], '2026-01-01'), null)
  assert.equal(valueAt(undefined, '2026-01-01'), null)
})

test('реальная доходность по Фишеру, не простая разность', () => {
  assert.ok(Math.abs(realRate(16.5, 6.1) - 9.8021) < 0.001) // 1,165 / 1,061 − 1
  assert.ok(Math.abs(realRate(5, 10) - -4.5454) < 0.001)
  assert.equal(realRate(10, 10), 0)
  assert.equal(realRate(null, 6), null)
  assert.equal(realRate(10, null), null)
  assert.equal(realRate(10, undefined), null)
})

test('подписи: знак, запятая, минус настоящий', () => {
  assert.equal(signedPct(9.8021), '+9,8 %')
  assert.equal(signedPct(-1.24), '−1,2 %')
  assert.equal(signedPct(0), '0 %')
  assert.equal(signedPct(null), '—')
  assert.equal(plainPct(15.5), '15,5 %')
  assert.equal(plainPct(undefined), '—')
})

test('текущая инфляция — последний месяц; нет данных — null', () => {
  assert.equal(currentInflation({ cpi_yoy: cpi }), 6.1)
  assert.equal(currentInflation({}), null)
  assert.equal(currentInflation(undefined), null)
})

test('сводка: ставка на сегодня, инфляция последнего месяца, реальная ставка', () => {
  const s = macroSummary({ policy_rate: rate, cpi_yoy: cpi, core_yoy: [{ d: '2026-09-01', v: 5.6 }] }, '2026-11-12')
  assert.equal(s.policy, 15.5)
  assert.equal(s.cpi, 6.1)
  assert.equal(s.cpiMonth, '2026-09')
  assert.equal(s.core, 5.6)
  assert.equal(s.overnightCD, null)
  assert.ok(Math.abs(s.realPolicy - ((1.155 / 1.061 - 1) * 100)) < 1e-9)
})

test('график: пустой без данных; ставка ступенькой тянется до сегодня, инфляция ломаной', () => {
  assert.equal(buildMacroChart({ series: {}, ids: ['policy_rate'], years: 5, todayIso: '2026-11-12' }).empty, true)
  const c = buildMacroChart({ series: { policy_rate: rate, cpi_yoy: cpi }, ids: ['policy_rate', 'cpi_yoy'], years: 5, todayIso: '2026-11-12' })
  assert.equal(c.empty, false)
  const p = c.lines.find((l) => l.id === 'policy_rate').d
  assert.match(p, /^M[\d.]+ [\d.]+(H[\d.]+V[\d.]+)+H[\d.]+$/) // старт, ступеньки смены ставки, хвост до сегодня
  assert.ok(p.includes('V'))
  const i = c.lines.find((l) => l.id === 'cpi_yoy').d
  assert.equal((i.match(/L/g) || []).length, 2) // три месяца — два отрезка
  assert.ok(c.yTicks.length >= 2 && c.yTicks[0].y > c.yTicks[c.yTicks.length - 1].y, 'большее значение выше на экране')
  assert.ok(c.todayX > c.pad.l && c.todayX <= c.width - c.pad.r + 0.01)
  assert.equal(c.zeroY === null || typeof c.zeroY === 'number', true)
  for (const l of c.lines) assert.ok(!l.d.includes('NaN'), l.d)
})

test('график: окно «5 лет» берёт значение ставки, действовавшее на левом краю', () => {
  const c = buildMacroChart({ series: { policy_rate: [{ d: '2010-01-01', v: 7 }, { d: '2026-06-01', v: 15 }] }, ids: ['policy_rate'], years: 5, todayIso: '2026-11-12' })
  const d = c.lines[0].d
  assert.ok(d.startsWith('M46.0 '), d) // линия начинается у левого края, а не пропадает
  assert.ok(d.includes('V'))
})

test('сортировка по «Реальной»: минус и ноль — это значения, пусто (null) уходит в конец', () => {
  const rows = [{ id: 'a', real: -1.5 }, { id: 'b', real: null }, { id: 'c', real: 0 }, { id: 'd', real: 9.8 }]
  assert.deepEqual(sortBonds(rows, 'real', 'asc').map((r) => r.id), ['a', 'c', 'd', 'b'])
  assert.deepEqual(sortBonds(rows, 'real', 'desc').map((r) => r.id), ['d', 'c', 'a', 'b'])
})

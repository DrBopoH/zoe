import test from 'node:test'
import assert from 'node:assert/strict'
import { valueAt, realRate, signedPct, plainPct, currentInflation, macroSummary, buildMacroChart, macroWindow, macroRanges, yearsFrom, isoAt, goldUsd, annualChange, unitText, axisText } from '../src/utils/macro.js'
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

const chartOf = (series, ids, years, todayIso = '2026-11-12', yRange = null) => {
  const view = macroWindow({ series, ids, todayIso, years })
  const r = yRange || macroRanges({ series, ids, view, todayIso }).auto
  return { view, chart: buildMacroChart({ series, ids, view, yRange: r, todayIso }) }
}

test('график: пустой без данных; ставка ступенькой тянется до сегодня, инфляция ломаной', () => {
  assert.equal(chartOf({}, ['policy_rate'], 5).chart.empty, true)
  const { chart: c } = chartOf({ policy_rate: rate, cpi_yoy: cpi }, ['policy_rate', 'cpi_yoy'], 5)
  assert.equal(c.empty, false)
  const p = c.lines.find((l) => l.id === 'policy_rate').d
  assert.match(p, /^M[\d.]+ [\d.]+(H[\d.]+V[\d.]+)+H[\d.]+$/) // старт, ступеньки смены ставки, хвост до сегодня
  const i = c.lines.find((l) => l.id === 'cpi_yoy').d
  assert.equal((i.match(/L/g) || []).length, 2) // три месяца — два отрезка
  assert.ok(c.yTicks.length >= 2 && c.yTicks[0].y > c.yTicks[c.yTicks.length - 1].y, 'большее значение выше на экране')
  assert.ok(c.todayX > c.pad.l && c.todayX <= c.width - c.pad.r + 0.01)
  for (const l of c.lines) assert.ok(!l.d.includes('NaN'), l.d)
})

test('окно: годы от сегодня, «5 лет» не шире 5 лет, «всё» — с первой точки; даты и годы переводятся друг в друга', () => {
  const s = { policy_rate: [{ d: '2010-01-01', v: 7 }, { d: '2026-06-01', v: 15 }] }
  const w5 = macroWindow({ series: s, ids: ['policy_rate'], todayIso: '2026-11-12', years: 5 })
  assert.equal(w5.x0, -5)
  assert.ok(w5.x1 > 0 && w5.x1 < 0.2)
  const all = macroWindow({ series: s, ids: ['policy_rate'], todayIso: '2026-11-12', years: 0 })
  assert.ok(all.x0 < -16 && all.x0 > -17)
  assert.ok(Math.abs(yearsFrom('2025-11-12', '2026-11-12') + 1) < 0.01)
  assert.equal(isoAt(yearsFrom('2024-03-05', '2026-11-12'), '2026-11-12'), '2024-03-05')
})

test('окно «5 лет» берёт значение ставки, действовавшее на левом краю', () => {
  const { chart: c } = chartOf({ policy_rate: [{ d: '2010-01-01', v: 7 }, { d: '2026-06-01', v: 15 }] }, ['policy_rate'], 5)
  const d = c.lines[0].d
  assert.ok(d.startsWith('M'), d)
  assert.ok(d.includes('V'))
})

test('вертикаль: один сбойный месяц не сплющивает график, но остаётся достижимым в крайностях', () => {
  const pts = []
  for (let m = 0; m < 120; m++) pts.push({ d: `${2017 + Math.floor(m / 12)}-${String((m % 12) + 1).padStart(2, '0')}-01`, v: 5 + (m % 7) })
  pts[40].v = -50 // выброс
  const view = macroWindow({ series: { core_yoy: pts }, ids: ['core_yoy'], todayIso: '2026-11-12', years: 10 })
  const r = macroRanges({ series: { core_yoy: pts }, ids: ['core_yoy'], view, todayIso: '2026-11-12' })
  assert.ok(r.auto.lo > -10, `auto.lo ${r.auto.lo}`) // выброс срезан
  assert.ok(r.ext.lo < -50, `ext.lo ${r.ext.lo}`) // но достижим сдвигом
  assert.ok(r.auto.hi < 20)
})

test('сортировка по «Реальной»: минус и ноль — это значения, пусто (null) уходит в конец', () => {
  const rows = [{ id: 'a', real: -1.5 }, { id: 'b', real: null }, { id: 'c', real: 0 }, { id: 'd', real: 9.8 }]
  assert.deepEqual(sortBonds(rows, 'real', 'asc').map((r) => r.id), ['a', 'c', 'd', 'b'])
  assert.deepEqual(sortBonds(rows, 'real', 'desc').map((r) => r.id), ['d', 'c', 'a', 'b'])
})

test('золото в долларах: гривенная цена на курс той же даты', () => {
  const g = goldUsd({ xau_uah: [{ d: '2026-01-02', v: 200000 }, { d: '2026-01-05', v: 210000 }], usd_uah: [{ d: '2026-01-01', v: 40 }, { d: '2026-01-05', v: 42 }] })
  assert.deepEqual(g, [{ d: '2026-01-02', v: 5000 }, { d: '2026-01-05', v: 5000 }])
  assert.deepEqual(goldUsd({}), [])
})

test('среднегодовое изменение ряда: сложный процент, нет данных так далеко — null', () => {
  const u = [{ d: '2016-11-01', v: 25 }, { d: '2026-11-12', v: 50 }]
  assert.ok(Math.abs(annualChange(u, '2026-11-12', 10) - 7.18) < 0.05)
  assert.equal(annualChange(u, '2026-11-12', 20), null)
  assert.equal(annualChange([], '2026-11-12', 5), null)
})

test('подписи значений по единице', () => {
  assert.equal(unitText(45.3222, '₴'), '45,32 ₴')
  assert.equal(unitText(183562, '₴'), '184 тыс. ₴')
  assert.equal(unitText(4050.4, '$'), '4050 $')
  assert.equal(unitText(10.3), '10,3 %')
  assert.equal(axisText(30), '30 %')
  assert.equal(axisText(150000, '₴'), '150к ₴')
  assert.equal(unitText(null, '₴'), '—')
})

test('логарифмическая шкала: равные проценты — равные расстояния, деления — степени десяти, нули не рисуются', () => {
  const today = '2026-11-12'
  const series = { px: [{ d: '2006-11-12', v: 10 }, { d: '2016-11-12', v: 100 }, { d: '2026-11-12', v: 1000 }] }
  const view = { x0: -20, x1: 0.05 }
  const r = macroRanges({ series, ids: ['px'], view, todayIso: today, floor: null, log: true })
  const chart = buildMacroChart({ series, ids: ['px'], view, yRange: r.auto, todayIso: today, scale: 'log' })
  const ys = Object.fromEntries(chart.yTicks.map((t) => [t.v, t.y]))
  assert.ok([10, 100, 1000].every((v) => v in ys))
  const [a, b, c] = [ys[10], ys[100], ys[1000]]
  assert.ok(Math.abs((a - b) - (b - c)) < 1e-6) // равные кратности — равные расстояния
  assert.equal(chart.zeroY, null)
  const withZero = { px: [{ d: '2006-11-12', v: 0 }, ...series.px] }
  const c2 = buildMacroChart({ series: withZero, ids: ['px'], view, yRange: r.auto, todayIso: today, scale: 'log' })
  assert.ok(!c2.lines[0].d.includes('NaN') && !c2.lines[0].d.includes('Infinity'))
})

test('градиент по значению: вектор от значения from до to в пикселях, у обычных рядов градиента нет', () => {
  const today = '2026-11-12'
  const series = { crypto_fng: [{ d: '2026-01-01', v: 20 }, { d: '2026-11-01', v: 80 }], us_unrate: [{ d: '2026-01-01', v: 4 }, { d: '2026-11-01', v: 5 }] }
  const view = { x0: -1, x1: 0.05 }
  const chart = buildMacroChart({ series, ids: ['crypto_fng', 'us_unrate'], view, yRange: { lo: 0, hi: 100 }, todayIso: today })
  const [fng, un] = chart.lines
  assert.ok(fng.gradient && fng.gradient.y1 > fng.gradient.y2) // 0 внизу, 100 вверху
  assert.equal(fng.gradient.stops.length, 3)
  assert.equal(un.gradient, null)
})

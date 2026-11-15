import test from 'node:test'
import assert from 'node:assert/strict'
import { priceIndex, purchasingPower, realValue, realRateIndex, priceLevel, pppDeviation, incomeRatio, salaryInIntl, bigMacRows, burgersPerMonth } from '../src/utils/ppp.js'

// Помесячный индекс США (значения CPIAUCSL: январь 2000 — 169,3; октябрь 2026 — 337,476) с равным ростом между ними
const us = [{ d: '2000-01-01', v: 169.3 }, { d: '2013-06-01', v: 232.0 }, { d: '2026-10-01', v: 337.476 }]

test('ворота M16: доллар 2000 → 2026 → обратно совпадает с накопленной инфляцией США ±0,1 %', () => {
  const pp = purchasingPower(us, '2000-01-01', '2026-10-01')
  assert.ok(Math.abs(pp.inflation - (337.476 / 169.3 - 1) * 100) < 0.1)
  const there = realValue(1, us, '2000-01-01', '2026-10-01') // 1 $ 2000 года в ценах 2026
  const back = realValue(there, us, '2026-10-01', '2000-01-01') // и обратно
  assert.ok(Math.abs(back - 1) < 0.001)
  assert.ok(Math.abs(there - 337.476 / 169.3) / (337.476 / 169.3) < 0.001)
  // 1 $ 2026 года покупает столько, сколько 0,50 $ 2000 года
  assert.ok(Math.abs(pp.ratio - 169.3 / 337.476) < 1e-9)
})

test('индекс цены: узлы точные, между узлами — геометрически, до первой точки null', () => {
  const pts = [{ d: '2020-07-01', v: 100 }, { d: '2022-07-01', v: 121 }]
  assert.equal(priceIndex(pts, '2020-07-01'), 100)
  assert.ok(Math.abs(priceIndex(pts, '2021-07-01') - 110) < 0.2) // ровно 10 % в год
  assert.equal(priceIndex(pts, '2019-01-01'), null)
  assert.equal(priceIndex([], '2020-01-01'), null)
})

test('после последней точки: по заданной инфляции или по темпу последних двух точек', () => {
  const pts = [{ d: '2020-07-01', v: 100 }, { d: '2021-07-01', v: 110 }]
  assert.ok(Math.abs(priceIndex(pts, '2022-07-01', 0.2) - 132) < 0.1)
  assert.ok(Math.abs(priceIndex(pts, '2022-07-01') - 121) < 0.2)
  assert.equal(priceIndex([{ d: '2020-07-01', v: 100 }], '2024-07-01'), 100)
})

test('накопленная и среднегодовая инфляция', () => {
  const pts = [{ d: '2020-07-01', v: 100 }, { d: '2022-07-01', v: 121 }]
  const pp = purchasingPower(pts, '2020-07-01', '2022-07-01')
  assert.ok(Math.abs(pp.inflation - 21) < 1e-9)
  assert.ok(Math.abs(pp.perYear - 10) < 0.1)
  assert.equal(purchasingPower(pts, '2010-01-01', '2022-07-01'), null)
})

test('реальный курс: курс вырос ровно на инфляционную разницу — индекс 100; опережает — выше', () => {
  const fx = [{ d: '2020-07-01', v: 10 }, { d: '2021-07-01', v: 12 }]
  const cpi = [{ d: '2020-07-01', v: 100 }, { d: '2021-07-01', v: 120 }]
  const usCpi = [{ d: '2020-07-01', v: 100 }, { d: '2021-07-01', v: 100 }]
  assert.ok(Math.abs(realRateIndex(fx, cpi, usCpi, '2020-07-01')[1].v - 100) < 1e-9)
  const faster = realRateIndex([{ d: '2020-07-01', v: 10 }, { d: '2021-07-01', v: 15 }], cpi, usCpi, '2020-07-01')
  assert.ok(faster[1].v > 100)
  assert.deepEqual(realRateIndex(fx, cpi, usCpi, '2000-01-01'), [])
})

test('уровень цен и отклонение от PPP (Украина 2025: 11,86 ÷ 41,69)', () => {
  const conv = [{ d: '2025-07-01', v: 11.8646 }], fx = [{ d: '2025-07-01', v: 41.689 }]
  assert.ok(Math.abs(priceLevel(conv, fx, '2025-07-01') - 0.2846) < 1e-3)
  assert.ok(Math.abs(pppDeviation(conv, fx, '2025-07-01') - -71.5) < 0.1)
  assert.equal(priceLevel([], fx, '2025-07-01'), null)
})

test('доходы стран и оклад в международных долларах', () => {
  const r = incomeRatio([{ d: '2025-07-01', v: 90000 }], [{ d: '2025-07-01', v: 18000 }], [{ d: '2025-07-01', v: 90000 }], [{ d: '2025-07-01', v: 6000 }], '2025-07-01')
  assert.equal(r.ppp, 5)
  assert.equal(r.nominal, 15)
  assert.ok(Math.abs(salaryInIntl(35000, [{ d: '2025-07-01', v: 11.8646 }], '2025-07-01') - 2950) < 1)
  assert.equal(salaryInIntl(1000, [], '2025-07-01'), null)
})

test('Big Mac по странам и бургеры в месяц', () => {
  const rows = bigMacRows({ bm_usd_ua: [{ d: '2026-01-01', v: 3.5 }, { d: '2026-07-01', v: 3.7 }], bm_raw_ua: [{ d: '2026-07-01', v: -38 }] }, ['ua', 'us'])
  assert.deepEqual(rows[0], { cc: 'ua', price: 3.7, raw: -38, date: '2026-07-01' })
  assert.deepEqual(rows[1], { cc: 'us', price: null, raw: null, date: null })
  assert.equal(burgersPerMonth(740, 3.7), 200)
  assert.equal(burgersPerMonth(0, 3.7), null)
})

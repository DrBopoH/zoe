import test from 'node:test'
import assert from 'node:assert/strict'
import { realRateRows, meanStd, zAt, stressAt, stressSeries, stressLevel, signalRows, signalTally, claimsRise, regime } from '../src/utils/scorecard.js'
import { SERIES } from '../src/utils/macro.js'

const TODAY = '2026-11-15'
const weekly = (vals, start = '2025-01-05') => vals.map((v, i) => ({ d: new Date(Date.parse(start) + i * 7 * 864e5).toISOString().slice(0, 10), v }))
const monthly = (vals, startYear = 2024) => vals.map((v, i) => ({ d: `${startYear + Math.floor(i / 12)}-${String((i % 12) + 1).padStart(2, '0')}-01`, v }))

test('реальные ставки стран: Фишер, кэрри к ФРС, сортировка по убыванию', () => {
  const countries = [
    { cc: 'us', name: 'США', metrics: { policy: 'fed', cpi: 'us_c' } },
    { cc: 'ua', name: 'Украина', metrics: { policy: 'ua_p', cpi: 'ua_c' } },
    { cc: 'jp', name: 'Япония', metrics: { policy: 'jp_p', cpi: 'jp_c' } },
    { cc: 'xx', name: 'Без инфляции', metrics: { policy: 'xx_p' } }
  ]
  const series = {
    fed: [{ d: '2026-10-01', v: 4 }], us_c: [{ d: '2026-10-01', v: 3 }],
    ua_p: [{ d: '2026-10-01', v: 15 }], ua_c: [{ d: '2026-10-01', v: 10 }],
    jp_p: [{ d: '2026-10-01', v: 0.5 }], jp_c: [{ d: '2026-10-01', v: 3 }]
  }
  const rows = realRateRows(countries, series, TODAY)
  assert.deepEqual(rows.map((r) => r.cc), ['ua', 'us', 'jp'])
  assert.equal(Math.round(rows[0].real * 100) / 100, 4.55)
  assert.equal(rows[0].carry, 11)
  assert.equal(rows[1].carry, 0)
  assert.ok(rows[2].real < 0)
})

test('z-оценка и индекс стресса: нужно минимум три компонента, кривая считается с обратным знаком', () => {
  const calm = weekly(Array(30).fill(0).map((_, i) => 10 + (i % 3)))
  const st = meanStd(calm)
  assert.ok(st && st.std > 0)
  assert.equal(meanStd(weekly([1, 2, 3])), null)
  assert.equal(meanStd(weekly(Array(20).fill(5))), null)
  const high = zAt([{ d: '2026-01-01', v: 30 }], st, 1, '2026-02-01')
  assert.ok(high > 5)
  assert.ok(zAt([{ d: '2026-01-01', v: 30 }], st, -1, '2026-02-01') < -5)

  const mk = (base, last) => weekly([...Array(29).fill(0).map((_, i) => base + (i % 3)), last])
  const series = { us_vix: mk(15, 40), us_hy: mk(3, 9), us_nfci: mk(-0.5, 1) }
  const iso = series.us_vix[29].d
  const s = stressAt(series, iso)
  assert.equal(s.parts.length, 3)
  assert.ok(s.value > 1.5, String(s.value))
  assert.equal(stressLevel(s.value), 'stress')
  assert.equal(stressAt({ us_vix: series.us_vix, us_hy: series.us_hy }, iso), null)
  const ser = stressSeries(series)
  assert.ok(ser.length > 0 && ser[ser.length - 1].d === iso)
  assert.ok(SERIES.stress_idx)
})

test('уровни стресса', () => {
  assert.equal(stressLevel(-1), 'calm')
  assert.equal(stressLevel(0), 'normal')
  assert.equal(stressLevel(1), 'tense')
  assert.equal(stressLevel(2), 'stress')
  assert.equal(stressLevel(null), null)
})

test('светофор сигналов: инверсия, Сам, кредит, VIX; без данных ряда нет', () => {
  const series = {
    us_spread: [{ d: '2026-11-01', v: -0.3 }],
    us_unrate: monthly([...Array(15).fill(4), 4.4, 4.8, 5.2]),
    us_hy: [{ d: '2026-11-01', v: 3 }],
    us_vix: [{ d: '2026-11-01', v: 35 }],
    us_sentiment: [{ d: '2026-10-01', v: 70 }]
  }
  const rows = Object.fromEntries(signalRows(series, TODAY).map((r) => [r.key, r.state]))
  assert.equal(rows.curve, 'alarm')
  assert.equal(rows.sahm, 'alarm')
  assert.equal(rows.hy, 'ok')
  assert.equal(rows.vix, 'alarm')
  assert.equal(rows.sentiment, 'warn')
  assert.equal(rows.nfci, undefined)
  assert.deepEqual(signalTally(signalRows(series, TODAY)), { ok: 1, warn: 1, alarm: 3, total: 5 })
  assert.deepEqual(signalRows({}, TODAY), [])
})

test('заявки на пособие: рост среднего за 4 недели над минимумом года', () => {
  assert.equal(claimsRise(weekly(Array(20).fill(200))), null)
  assert.equal(claimsRise(weekly(Array(60).fill(200))), 0)
  assert.equal(claimsRise(weekly([...Array(56).fill(200), 300, 300, 300, 300])), 50)
})

test('режим экономики: правила и объяснение', () => {
  const base = (gdp, cpiNow, cpiBefore, unrate = Array(18).fill(4)) => ({
    us_gdp: [{ d: '2026-07-01', v: gdp }],
    us_cpi_yoy: [{ d: '2026-04-01', v: cpiBefore }, { d: '2026-10-01', v: cpiNow }],
    us_unrate: monthly(unrate)
  })
  assert.equal(regime(base(2.5, 2.2, 2.4), TODAY).key, 'goldilocks')
  assert.equal(regime(base(2.5, 4.5, 4.4), TODAY).key, 'overheating')
  assert.equal(regime(base(-1, 2.2, 2.4), TODAY).key, 'recession')
  assert.equal(regime(base(-1, 5, 4.8), TODAY).key, 'stagflation')
  assert.equal(regime(base(0.8, 2.6, 3.4), TODAY).key, 'disinflation')
  const sahm = regime(base(2, 2.2, 2.2, [...Array(15).fill(4), 4.4, 4.8, 5.2]), TODAY)
  assert.equal(sahm.key, 'recession')
  assert.ok(sahm.why.length >= 3)
  assert.equal(regime({}, TODAY), null)
})

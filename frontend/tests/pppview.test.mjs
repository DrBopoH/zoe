import test from 'node:test'
import assert from 'node:assert/strict'
import { PPP_COUNTRIES, countryOf, cpiPoints, convPoints, startIso, growthOf, pppModel, levelRows, burgerRows, compareCountries, realReturn } from '../src/utils/pppview.js'
import { SERIES } from '../src/utils/macro.js'
import { MARKET_SECTIONS, feedOf } from '../src/utils/marketNav.js'

const TODAY = '2026-11-16'
// Украина: цены ×2 за 10 лет (07.2015 → 07.2025), курс 25 → 40; США: ×1,3; PPP-коэффициент 12, 1 $ = 40 ₴
const series = {
  ppp_cpi_ua: [{ d: '2015-07-01', v: 100 }, { d: '2025-07-01', v: 200 }],
  ppp_cpi_de: [{ d: '2015-07-01', v: 100 }, { d: '2025-07-01', v: 120 }],
  ppp_us_cpi: [{ d: '2015-07-01', v: 100 }, { d: '2025-07-01', v: 130 }],
  ppp_fx_ua: [{ d: '2015-07-01', v: 25 }, { d: '2025-07-01', v: 40 }],
  ppp_fx_ez: [{ d: '2025-07-01', v: 0.9 }],
  ppp_fx_pl: [{ d: '2025-07-01', v: 4 }],
  ppp_conv_ua: [{ d: '2025-07-01', v: 12 }],
  ppp_conv_de: [{ d: '2025-07-01', v: 0.72 }],
  ppp_conv_pl: [{ d: '2025-07-01', v: 2 }],
  ppp_gdp_ua: [{ d: '2025-07-01', v: 18000 }], ppp_gdpn_ua: [{ d: '2025-07-01', v: 6000 }],
  ppp_gdp_pl: [{ d: '2025-07-01', v: 54000 }], ppp_gdpn_pl: [{ d: '2025-07-01', v: 28000 }],
  ppp_gdp_us: [{ d: '2025-07-01', v: 90000 }], ppp_gdpn_us: [{ d: '2025-07-01', v: 90000 }],
  bm_usd_ua: [{ d: '2026-07-01', v: 3.7 }], bm_usd_us: [{ d: '2026-07-01', v: 6 }], bm_raw_ua: [{ d: '2026-07-01', v: -38 }]
}

test('страны: еврозона берёт немецкие цены и PPP, США — помесячный индекс', () => {
  assert.equal(cpiPoints(series, 'ez'), series.ppp_cpi_de)
  assert.equal(convPoints(series, 'ez'), series.ppp_conv_de)
  assert.equal(cpiPoints(series, 'us'), series.ppp_us_cpi)
  assert.equal(countryOf('ua').sym, '₴')
  assert.equal(startIso('2026-11-16', 10), '2016-07-01')
})

test('темп инфляции для продолжения: у Украины — годовая инфляция НБУ, у остальных нет', () => {
  assert.equal(growthOf('ua', { cpi_yoy: [{ d: '2026-10-01', v: 8 }] }), 0.08)
  assert.equal(growthOf('ua', {}), null)
  assert.equal(growthOf('pl', { cpi_yoy: [{ d: '2026-10-01', v: 8 }] }), null)
})

test('модель: деньги теряют покупательную способность, суммы тогда и сегодня согласованы', () => {
  const m = pppModel({ series, macro: {}, cc: 'ua', years: 10, today: '2025-07-01', amount: 1000 })
  assert.equal(m.from, '2015-07-01')
  assert.ok(Math.abs(m.pp.inflation - 100) < 1e-6)
  assert.ok(Math.abs(m.needNow - 2000) < 1e-6)
  assert.ok(Math.abs(m.worthThen - 500) < 1e-6)
  assert.equal(m.power[0].v, 100)
  assert.ok(Math.abs(m.power[m.power.length - 1].v - 50) < 1e-6)
  // курс вырос 25 → 40 (+60 %), цены Украины ×2, США ×1,3 (разница инфляций ×1,54): гривна ослабла чуть сильнее инфляционной разницы → индекс выше 100
  const r = m.rer[m.rer.length - 1].v
  assert.ok(Math.abs(r - (40 / 25) * 1.3 / 2 * 100) < 1e-6)
  assert.ok(r > 100)
})

test('модель: США — без реального курса к доллару; нет данных — пусто', () => {
  assert.deepEqual(pppModel({ series, macro: {}, cc: 'us', years: 10, today: '2025-07-01' }).rer, [])
  const m = pppModel({ series: {}, macro: {}, cc: 'ua', years: 10, today: TODAY })
  assert.equal(m.pp, null)
  assert.equal(m.needNow, null)
  assert.deepEqual(m.power, [])
})

test('уровень цен: дорогие сверху, у США 1, без данных страны пропущены', () => {
  const rows = levelRows(series, TODAY)
  assert.equal(rows[0].cc, 'us')
  const ua = rows.find((r) => r.cc === 'ua')
  assert.ok(Math.abs(ua.level - 0.3) < 1e-9)
  assert.ok(Math.abs(ua.dev - -70) < 1e-9)
  assert.equal(rows.find((r) => r.cc === 'pl').level, 0.5)
  assert.ok(!rows.some((r) => r.cc === 'jp'))
})

test('Big Mac: цена, отклонение, бургеров в месяц на среднедушевой доход (6000 ÷ 12 ÷ 3,7)', () => {
  const rows = burgerRows(series, TODAY)
  assert.deepEqual(rows.map((r) => r.cc), ['us', 'ua'])
  const ua = rows[1]
  assert.equal(ua.raw, -38)
  assert.ok(Math.abs(ua.perMonth - 500 / 3.7) < 1e-9)
})

test('две страны: оклад по PPP и по рыночному курсу', () => {
  // 36 000 ₴ при PPP 12 = 3 000 междунар. $; в Польше (PPP 2) это 6 000 zł; по рынку: 36 000 / 40 = 900 $ = 3 600 zł (курс 4)
  const r = compareCountries({ series, a: 'ua', b: 'pl', salaryA: 36000, today: TODAY })
  assert.equal(r.intl, 3000)
  assert.equal(r.inB, 6000)
  assert.equal(r.usdA, 900)
  assert.equal(r.inBMarket, 3600)
  assert.ok(Math.abs(r.income.ppp - 18000 / 54000) < 1e-9)
  assert.equal(compareCountries({ series: {}, a: 'ua', b: 'pl', salaryA: 1, today: TODAY }).inB, null)
})

test('реальная доходность: 15 % при инфляции 10 % → 4,5 %; 5 % при 10 % — минус', () => {
  const a = realReturn({ nominalPct: 15, inflationPct: 10, years: 2, amount: 1000 })
  assert.ok(Math.abs(a.real - (1.15 / 1.1 - 1) * 100) < 1e-9)
  assert.ok(Math.abs(a.grown - 1322.5) < 1e-6)
  assert.ok(Math.abs(a.today - 1000 * Math.pow(1.15 / 1.1, 2)) < 1e-6)
  assert.equal(a.beats, true)
  assert.equal(realReturn({ nominalPct: 5, inflationPct: 10, years: 1, amount: 100 }).beats, false)
  assert.equal(realReturn({ nominalPct: NaN, inflationPct: 10, years: 1, amount: 100 }), null)
})

test('подписи рядов страницы и раздел в навигации', () => {
  for (const c of PPP_COUNTRIES) {
    assert.ok(SERIES[`pw_${c.cc}`]?.label && SERIES[`pw_${c.cc}`]?.color, `pw_${c.cc}`)
    if (c.cc !== 'us') assert.ok(SERIES[`rerusd_${c.cc}`]?.label, `rerusd_${c.cc}`)
  }
  assert.ok(MARKET_SECTIONS.includes('ppp'))
  assert.equal(feedOf('ppp'), 'ppp')
})

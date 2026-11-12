import test from 'node:test'
import assert from 'node:assert/strict'
import { capitalStructure, portfolioStats } from '../src/utils/capital.js'

const acc = [
  { id: 'a', currency_code: 980, balance: 1000 },
  { id: 'b', currency_code: 980, balance: 500.5 },
  { id: 'usd', currency_code: 840, balance: 99999 }, // валютный — не в капитале
  { id: 'icu', currency_code: 980, bank_key: 'icu', balance: 9000, portfolio: { kind: 'bonds', cash: 1000, invested: 8000 } },
  { id: 'bn', currency_code: 980, bank_key: 'binance', balance: 2500 }
]

test('структура капитала: слои, доли, валютные счета не входят', () => {
  const s = capitalStructure(acc)
  assert.equal(s.total, 13000.5)
  assert.deepEqual(s.parts.map((p) => [p.key, p.value]), [['cards', 1500.5], ['broker', 1000], ['bonds', 8000], ['crypto', 2500]])
  assert.equal(s.parts.find((p) => p.key === 'bonds').pct, 61.5)
  assert.equal(s.parts.reduce((x, p) => x + p.value, 0), s.total)
})

test('структура капитала: пустой слой скрыт, брокерский без состава — деньги, пустой список — нули', () => {
  const s = capitalStructure([{ id: 'icu', currency_code: 980, bank_key: 'icu', balance: 700 }])
  assert.deepEqual(s.parts.map((p) => p.key), ['broker'])
  assert.deepEqual(capitalStructure([]), { unit: '₴', total: 0, totalUah: 0, parts: [], currencies: [], skipped: 0 })
  assert.equal(capitalStructure([{ currency_code: 980, balance: 0 }]).parts.length, 0)
})

const pos = [
  { qty: 2, value: 2000, coupon_rate: 16, maturity: '2028-11-12', next_date: '2026-12-01', next_amount: 80 },
  { qty: 1, value: 1000, coupon_rate: 10, maturity: '2027-11-12', next_date: '2026-12-01', next_amount: 50 },
  { qty: 1, value: 500, no_terms: true },
  { qty: 1, value: 900, coupon_rate: 12, maturity: '2025-01-01' }, // погашена
  { qty: 0, value: 0, coupon_rate: 20, maturity: '2030-01-01' }
]

test('портфель ОВДП: средняя ставка и срок по стоимости, реальная доходность, ближайшая выплата', () => {
  const s = portfolioStats(pos, '2026-11-12', 10)
  assert.equal(s.count, 2)
  assert.equal(s.skipped, 2)
  assert.equal(s.value, 3000)
  assert.equal(s.rate, 14) // (2000·16 + 1000·10) / 3000 = 14
  assert.equal(s.years, 1.7) // (2000·2 + 1000·1) / 3000
  assert.ok(Math.abs(s.real - ((1.14 / 1.10 - 1) * 100)) < 1e-9)
  assert.deepEqual(s.next, { date: '2026-12-01', amount: 130 })
})

test('портфель ОВДП: нет облигаций со ставкой — null; нет инфляции — без реальной', () => {
  assert.equal(portfolioStats([], '2026-11-12'), null)
  assert.equal(portfolioStats([{ qty: 1, value: 500, no_terms: true }], '2026-11-12'), null)
  assert.equal(portfolioStats(pos, '2026-11-12').real, null)
})

import { toUsd, fxRates, bondDollarYield, dollarAssumptions } from '../src/utils/dollar.js'

test('капитал в долларах: доллар как есть, гривна и Binance по курсу, евро через гривну', () => {
  const rates = { usdUah: 40, eurUah: 44 }
  const s = capitalStructure([
    { id: 'u', currency_code: 980, balance: 4000 },
    { id: 'd', currency_code: 840, balance: 100 },
    { id: 'e', currency_code: 978, balance: 100 },
    { id: 'bn', currency_code: 980, bank_key: 'binance', balance: 4200, portfolio: { usdt_uah: 42 } },
    { id: 'g', currency_code: 959, balance: 1 }
  ], rates)
  assert.equal(s.unit, '$')
  assert.equal(s.total, 100 + 100 + 110 + 100)
  assert.equal(s.totalUah, 410 * 40)
  assert.equal(s.skipped, 1)
  assert.deepEqual(s.currencies.map((c) => [c.key, c.value]), [['UAH', 100], ['USD', 200], ['EUR', 110]])
})

test('без курса капитал как раньше — гривневые счета в гривне', () => {
  const s = capitalStructure(acc)
  assert.equal(s.unit, '₴')
  assert.equal(s.total, 13000.5)
})

test('доходность в долларах: UAH с поправкой на девальвацию, USD как есть, EUR через рост евро', () => {
  const a = bondDollarYield({ rate: 15, currency: 'UAH', devUsd: 10, usInflation: 3 })
  assert.ok(Math.abs(a.usd - (1.15 / 1.1 - 1) * 100) < 1e-9)
  assert.ok(Math.abs(a.realUsd - ((1 + a.usd / 100) / 1.03 - 1) * 100) < 1e-9)
  assert.equal(bondDollarYield({ rate: 4.5, currency: 'USD', usInflation: 3 }).usd, 4.5)
  assert.ok(Math.abs(bondDollarYield({ rate: 3, currency: 'EUR', eurGrowth: 2 }).usd - (1.03 * 1.02 - 1) * 100) < 1e-9)
  assert.equal(bondDollarYield({ rate: 15, currency: 'UAH' }).usd, null)
  assert.equal(bondDollarYield({ rate: 3, currency: 'EUR' }).usd, null)
})

test('курсы и допущения из макро-рядов', () => {
  const series = { usd_uah: [{ d: '2021-11-01', v: 25 }, { d: '2026-11-01', v: 41 }], eur_uah: [{ d: '2021-11-01', v: 30 }, { d: '2026-11-01', v: 45 }], us_cpi_yoy: [{ d: '2026-09-01', v: 3 }] }
  assert.deepEqual(fxRates(series), { usdUah: 41, eurUah: 45 })
  assert.equal(toUsd(978, 100, { usdUah: 40, eurUah: 44 }), 110)
  assert.equal(toUsd(980, 100, null), null)
  const a = dollarAssumptions(series, '2026-11-12', 5)
  assert.ok(a.devUsd > 0 && a.eurGrowth !== null && a.usInflation === 3)
})

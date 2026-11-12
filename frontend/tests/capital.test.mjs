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
  assert.deepEqual(capitalStructure([]), { total: 0, parts: [] })
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

import test from 'node:test'
import assert from 'node:assert/strict'
import { filterBonds, sortBonds, bondCells, currenciesOf, nextSort, isMatured } from '../src/utils/marketTable.js'

const L = [
  { id: 'UA4000A', name: 'ОВДП 2027', currency: 'UAH', maturity: '2027-03-10', coupon_rate: 16.5, yield: 15.2, nominal: 1000, last_auction: '2026-11-04' },
  { id: 'UA4000B', name: 'ОВДП USD', currency: 'USD', maturity: '2026-01-05', coupon_rate: 4.5, yield: 0, nominal: 1000, last_auction: '' },
  { id: 'UA4000C', name: 'Облигация без даты', currency: 'UAH', maturity: '', coupon_rate: 0, yield: 14, nominal: 0, last_auction: '' }
]

test('погашенные скрываются по умолчанию; без даты бумага действующая', () => {
  assert.equal(isMatured(L[1], '2026-11-12'), true)
  assert.equal(isMatured(L[2], '2026-11-12'), false)
  assert.deepEqual(filterBonds(L, { today: '2026-11-12' }).map((b) => b.id), ['UA4000A', 'UA4000C'])
  assert.equal(filterBonds(L, { today: '2026-11-12', hideMatured: false }).length, 3)
})

test('поиск по ISIN и названию без учёта регистра, фильтр по валюте', () => {
  assert.deepEqual(filterBonds(L, { q: 'ua4000b', hideMatured: false }).map((b) => b.id), ['UA4000B'])
  assert.deepEqual(filterBonds(L, { q: 'без даты' }).map((b) => b.id), ['UA4000C'])
  assert.deepEqual(filterBonds(L, { currency: 'USD', hideMatured: false }).map((b) => b.id), ['UA4000B'])
  assert.equal(filterBonds(null).length, 0)
})

test('сортировка: числа, строки, пустые всегда в конце в любом порядке', () => {
  assert.deepEqual(sortBonds(L, 'coupon_rate', 'asc').map((b) => b.id), ['UA4000B', 'UA4000A', 'UA4000C'])
  assert.deepEqual(sortBonds(L, 'coupon_rate', 'desc').map((b) => b.id), ['UA4000A', 'UA4000B', 'UA4000C'])
  assert.deepEqual(sortBonds(L, 'maturity', 'asc').map((b) => b.id), ['UA4000B', 'UA4000A', 'UA4000C'])
  assert.deepEqual(sortBonds(L, 'maturity', 'desc').map((b) => b.id), ['UA4000A', 'UA4000B', 'UA4000C'])
  assert.deepEqual(sortBonds(L, 'нет-такого').map((b) => b.id), ['UA4000A', 'UA4000B', 'UA4000C'])
  assert.equal(L[0].id, 'UA4000A') // исходный список не меняется
})

test('подписи ячеек', () => {
  assert.deepEqual(bondCells(L[0]), { id: 'UA4000A', name: 'ОВДП 2027', maturity: '10.03.2027', coupon_rate: '16,5 %', yield: '15,2 %', last_auction: '04.11.2026', nominal: '₴ 1 000' })
  const c = bondCells(L[2])
  assert.equal(c.maturity, '—'); assert.equal(c.coupon_rate, '—'); assert.equal(c.nominal, '—'); assert.equal(c.last_auction, '—')
  assert.equal(bondCells(L[1]).nominal.startsWith('$'), true)
})

test('доходность округляется до сотых', () => {
  assert.equal(bondCells({ id: 'X', yield: 15.299999999999999, coupon_rate: 3.14159, currency: 'USD' }).yield, '15,3 %')
  assert.equal(bondCells({ id: 'X', yield: 15.299999999999999, coupon_rate: 3.14159, currency: 'USD' }).coupon_rate, '3,14 %')
})

test('валюты списка и переключение сортировки', () => {
  assert.deepEqual(currenciesOf(L), ['UAH', 'USD'])
  assert.deepEqual(nextSort({ key: 'id', dir: 'asc' }, 'id'), { key: 'id', dir: 'desc' })
  assert.deepEqual(nextSort({ key: 'id', dir: 'desc' }, 'id'), { key: 'id', dir: 'asc' })
  assert.deepEqual(nextSort({ key: 'id', dir: 'desc' }, 'yield'), { key: 'yield', dir: 'asc' })
})

import { scoreBonds, scoreColor, rateOf } from '../src/utils/marketTable.js'

const T = '2026-01-01'
const S = [
  { id: 'A', currency: 'UAH', maturity: '2027-01-01', coupon_rate: 10, yield: 0 }, // 365 дн., ставка 10
  { id: 'B', currency: 'UAH', maturity: '2026-07-02', coupon_rate: 5, yield: 20 },  // ~182 дн., ставка 20 (аукцион главнее купона)
  { id: 'C', currency: 'UAH', maturity: '2025-12-01', coupon_rate: 30, yield: 0 },  // погашена — не оценивается
  { id: 'D', currency: 'UAH', maturity: '2027-01-01', coupon_rate: 0, yield: 0 },    // нет ставки
  { id: 'E', currency: 'UAH', maturity: '', coupon_rate: 9, yield: 0 },              // нет даты
  { id: 'U', currency: 'USD', maturity: '2026-07-02', coupon_rate: 4, yield: 0 }     // другая группа: сама себе 100 % по ставке
]

test('ставка бумаги: аукционная доходность главнее купона', () => {
  assert.equal(rateOf(S[1]), 20); assert.equal(rateOf(S[0]), 10); assert.equal(rateOf(S[3]), 0)
})

test('коэффициент: срок и ставка относительно лучшей бумаги валюты, среднее из двух', () => {
  const m = scoreBonds(S, T)
  assert.equal(m.has('C') || m.has('D') || m.has('E'), false)
  const a = m.get('A'), b = m.get('B')
  assert.equal(a.term, 100); assert.equal(a.rate, 50); assert.equal(a.score, 75)
  assert.equal(Math.round(b.term), 50); assert.equal(b.rate, 100); assert.equal(Math.round(b.score), 75)
  assert.equal(m.get('U').rate, 100) // доллар меряется только с долларами
  assert.equal(m.get('U').term, 100)
})

test('места и значки внутри валюты: 1, до 10, до 25, до 100, дальше без значка', () => {
  const list = Array.from({ length: 120 }, (_, i) => ({ id: 'X' + String(i).padStart(3, '0'), currency: 'UAH', maturity: '2027-01-01', coupon_rate: 100 - i * 0.5, yield: 0 }))
  const m = scoreBonds(list, T)
  assert.deepEqual([m.get('X000').rank, m.get('X000').tier, m.get('X000').tierLabel], [1, 'top1', 'Топ 1'])
  assert.equal(m.get('X001').tier, 'top10'); assert.equal(m.get('X009').tier, 'top10')
  assert.equal(m.get('X010').tier, 'top25'); assert.equal(m.get('X024').tier, 'top25')
  assert.equal(m.get('X025').tier, 'top100'); assert.equal(m.get('X099').tier, 'top100')
  assert.equal(m.get('X100').tier, '')
})

test('цвет: красный → жёлтый → зелёный, значения вне 0–100 зажимаются', () => {
  assert.match(scoreColor(0).text, /^hsl\(0 /)
  assert.match(scoreColor(50).text, /^hsl\(60 /)
  assert.match(scoreColor(100).text, /^hsl\(120 /)
  assert.match(scoreColor(250).text, /^hsl\(120 /)
  assert.match(scoreColor(-5).text, /^hsl\(0 /)
})

test('пустой список и мусор не падают', () => {
  assert.equal(scoreBonds(null, T).size, 0)
  assert.equal(scoreBonds([{ id: 'Z', currency: 'UAH', maturity: '2026-01-01', coupon_rate: 5 }], T).get('Z').term, 0) // погашение сегодня: срока нет
})

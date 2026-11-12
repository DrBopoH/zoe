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

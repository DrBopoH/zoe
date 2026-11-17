import test from 'node:test'
import assert from 'node:assert/strict'
import { normIsin, findMarketBond, fillFromMarket, ISIN_LEN } from '../src/utils/bondLookup.js'

const list = [
  { id: 'UA4000123456', name: 'ОВДП 2030', maturity: '2030-02-20', coupon_rate: 16.44, nominal: 1000 },
  { id: 'UA4000123457', name: 'Бескупонная', maturity: '2027-01-01', coupon_rate: 0, nominal: 1000 },
]

test('ISIN нормализуется, поиск только точный', () => {
  assert.equal(ISIN_LEN, 12)
  assert.equal(normIsin('  ua4000123456 '), 'UA4000123456')
  assert.equal(findMarketBond(list, 'ua4000123456').name, 'ОВДП 2030')
  assert.equal(findMarketBond(list, 'UA4000123'), null) // префикс — не совпадение
  assert.equal(findMarketBond(null, 'UA4000123456'), null)
})

test('форма заполняется из реестра: дата ДД.ММ.ГГГГ, ставка, название', () => {
  const f = fillFromMarket({ isin: 'UA4000123456', name: '', maturity: '', coupon_rate: null, nominal: 1000 }, list[0])
  assert.deepEqual(f, { isin: 'UA4000123456', name: 'ОВДП 2030', maturity: '20.02.2030', coupon_rate: 16.44, nominal: 1000 })
})

test('набранное руками не затирается, бескупонная ставку не подставляет', () => {
  const typed = { isin: 'x', name: 'Моё', maturity: '01.01.2031', coupon_rate: 10, nominal: 1000 }
  assert.deepEqual(fillFromMarket(typed, list[0]), typed)
  const z = fillFromMarket({ isin: 'x', name: '', maturity: '', coupon_rate: null, nominal: 1000 }, list[1])
  assert.equal(z.coupon_rate, null)
  assert.equal(z.maturity, '01.01.2027')
})

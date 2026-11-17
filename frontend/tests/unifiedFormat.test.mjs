import test from 'node:test'
import assert from 'node:assert/strict'
import { numText, fixedText } from '../src/utils/numfmt.js'
import { fmtQty } from '../src/utils/portfolioView.js'
import { priceText } from '../src/utils/commodities.js'
import { axisLabel } from '../src/utils/candleMath.js'

// Все экраны ходят в общие форматтеры: запятая, лишние нули не пишем, нет значения — «—».
test('доходность и проценты: целое без «,0» (раньше в таблице ОВДП было «15,0»)', () => {
  assert.equal(numText(15, 2), '15')
  assert.equal(numText(15.5, 2), '15,5')
  assert.equal(numText(12.345, 2), '12,35')
  assert.equal(numText(null, 2), '—')
  assert.equal(numText(NaN, 2), '—')
})

test('количество монеты: десятичная запятая и у стейблов, и у остальных; пыль — «0»', () => {
  assert.equal(fmtQty('USDT', 3), '3,00')
  assert.equal(fmtQty('USDT', 1234.567), '1234,57')
  assert.equal(fmtQty('BTC', 0.00123), '0,00123')
  assert.equal(fmtQty('BTC', 2), '2')
  assert.equal(fmtQty('ETH', 1.23456789123), '1,23456789')
  assert.equal(fmtQty('BTC', 4e-9), '0')
})

test('цена сырья и подписи шкалы остались прежними', () => {
  assert.equal(priceText(2.675), '2,68')
  assert.equal(priceText(9.4), '9,40')
  assert.equal(priceText(NaN), '—')
  assert.equal(axisLabel(9750, 250, 0, 12000), '9,75к')
  assert.equal(axisLabel(2.5e6, 5e5, 0, 3e6), '2,5М')
  assert.equal(fixedText(3, 2), '3,00')
})

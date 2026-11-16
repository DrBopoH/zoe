import test from 'node:test'
import assert from 'node:assert/strict'
import { numText, fixedText, intText, dmy } from '../src/utils/numfmt.js'

test('numText: запятая, лишние нули не пишутся, нет значения — тире', () => {
  assert.equal(numText(9.4), '9,4')
  assert.equal(numText(9.456), '9,46')
  assert.equal(numText(3), '3')
  assert.equal(numText(9.456, 1), '9,5')
  assert.equal(numText(-1.25, 1), '-1,2')
  assert.equal(numText(null), '—')
  assert.equal(numText(undefined), '—')
  assert.equal(numText(NaN), '—')
  assert.equal(numText(Infinity), '—')
})

test('fixedText: ровно столько знаков, сколько просили', () => {
  assert.equal(fixedText(3), '3,00')
  assert.equal(fixedText(1.23456, 4), '1,2346')
  assert.equal(fixedText(null), '—')
})

test('intText: целое с неразрывными пробелами между разрядами', () => {
  assert.equal(intText(1234567.4).replace(/\s/g, ' '), '1 234 567')
  assert.equal(intText(999), '999')
  assert.equal(intText(undefined), '—')
})

test('dmy: ГГГГ-ММ-ДД → ДД.ММ.ГГГГ, пусто — пустая строка', () => {
  assert.equal(dmy('2026-11-16'), '16.11.2026')
  assert.equal(dmy(''), '')
  assert.equal(dmy(null), '')
})

test('numText: малое отличное от нуля число не округляется до «0»', async () => {
  const { numText } = await import('../src/utils/numfmt.js')
  assert.equal(numText(0.0007), '0,0007')
  assert.equal(numText(0.0000142), '0,000014')
  assert.equal(numText(0.004, 2), '0,004')
  assert.equal(numText(0.5), '0,5')
  assert.equal(numText(0), '0')
  assert.equal(numText(-0.0007), '-0,0007')
})

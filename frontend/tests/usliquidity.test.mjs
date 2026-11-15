import test from 'node:test'
import assert from 'node:assert/strict'
import { SERIES, billionsText, creditLevel } from '../src/utils/macro.js'

test('у каждого ряда ликвидности и кредита есть подпись и цвет', () => {
  for (const id of ['us_m2', 'us_m2_yoy', 'us_fedbs', 'us_rrp', 'us_tga', 'us_netliq', 'us_sofr', 'us_hy', 'us_ig', 'us_nfci', 'us_usd', 'us_debt_gdp']) {
    assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
  }
})

test('млрд долларов: от тысячи — триллионы, иначе миллиарды', () => {
  assert.equal(billionsText(5900), '5,9 трлн $')
  assert.equal(billionsText(6758.867), '6,76 трлн $')
  assert.equal(billionsText(480.4), '480 млрд $')
  assert.equal(billionsText(0.325), '0 млрд $')
  assert.equal(billionsText(-1500), '-1,5 трлн $')
  assert.equal(billionsText(null), '—')
  assert.equal(billionsText(NaN), '—')
})

test('кредитный спред словами', () => {
  assert.equal(creditLevel(3.2), 'calm')
  assert.equal(creditLevel(4), 'wary')
  assert.equal(creditLevel(6), 'wary')
  assert.equal(creditLevel(7.5), 'stress')
  assert.equal(creditLevel(null), null)
  assert.equal(creditLevel(undefined), null)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { LAB_BADGES } from '../src/utils/labLedger.js'
import { BADGES, badgeOf, badgeTotals, entrySigned, withCash } from '../src/utils/glovoLedger.js'

test('бейджи лаба: Расход, Убыток, Продажа; ключи латиницей, стороны уход/уход/приход', () => {
  assert.deepEqual(LAB_BADGES.map((b) => b.key), ['expense', 'loss', 'sale'])
  for (const b of LAB_BADGES) assert.match(b.key, /^[a-z][a-z0-9-]{0,15}$/)
  assert.deepEqual(LAB_BADGES.map((b) => b.side), ['out', 'out', 'in'])
})

test('наборы бейджей независимы: итоги и знак записи берутся из набора страницы', () => {
  const tx = [
    { id: 'a', time: '2026-11-10T10:00:00', amount: -720 },
    { id: 'b', time: '2026-11-11T10:00:00', amount: -300 },
    { id: 'c', time: '2026-11-12T10:00:00', amount: 500 }
  ]
  const marks = new Map([['a', 'expense'], ['b', 'loss'], ['c', 'sale']])
  const t = badgeTotals(tx, marks, LAB_BADGES)
  assert.deepEqual(Object.keys(t), ['expense', 'loss', 'sale'])
  assert.equal(t.expense.sum, -720)
  assert.equal(t.sale.sum, 500)
  // «sale» не бейдж Glovo: там такая пометка не считается
  assert.equal(badgeTotals(tx, marks, BADGES).expense.sum, -720)
  assert.equal(badgeOf('sale'), null)
  assert.equal(badgeOf('sale', LAB_BADGES).label, 'Продажа')
  assert.equal(entrySigned({ badge: 'sale', amount: 100 }, LAB_BADGES), 100)
  assert.equal(entrySigned({ badge: 'sale', amount: 100 }), 100) // неизвестный бейдж Glovo — считается приходом, но withCash его отбрасывает
  assert.equal(withCash(tx, marks, [{ id: 'x', day: '2026-11-10', badge: 'sale', amount: 5 }]).transactions, tx)
  assert.equal(withCash(tx, marks, [{ id: 'x', day: '2026-11-10', badge: 'sale', amount: 5 }], LAB_BADGES).transactions.length, 4)
})

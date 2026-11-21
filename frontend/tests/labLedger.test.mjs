import test from 'node:test'
import assert from 'node:assert/strict'
import { LAB_BADGES, cleanCard, cardTotal, cardGrams, cardPerGram, cardsByTx, cardSplit, cardNames, cardTitle } from '../src/utils/labLedger.js'
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

test('cleanCard: числа, запятая, обязательные поля', () => {
  const c = cleanCard({ substance: ' мальтодекстрин ', maker: ' ', net_g: '1000', price: '96,5', qty: '5', note: '' })
  assert.deepEqual(c, { substance: 'мальтодекстрин', maker: '', net_g: 1000, price: 96.5, qty: 5, note: '' })
  assert.equal(cleanCard({ substance: '', net_g: 1, price: 1, qty: 1 }), null)
  assert.equal(cleanCard({ substance: 'x', net_g: 0, price: 1, qty: 1 }), null)
  assert.equal(cleanCard({ substance: 'x', net_g: 1, price: -1, qty: 1 }), null)
  assert.equal(cleanCard({ substance: 'x', net_g: 1, price: 1, qty: 0 }), null)
  assert.equal(cleanCard({ substance: 'x', net_g: 1, price: 1, qty: 1.5 }), null)
  assert.equal(cleanCard({ substance: 'x', net_g: 1, price: 0, qty: 1 }).price, 0) // бесплатный образец — карточка
})

test('карточка: 96 ₴ × 5 шт по 1000 г — 480 ₴ и 5000 г, грамм по 0,096 ₴; цены разных карточек не смешиваются', () => {
  const a = { id: 'a', tx_id: 't1', substance: 'мальтодекстрин', maker: 'А', net_g: 1000, price: 96, qty: 5 }
  const b = { id: 'b', tx_id: 't1', substance: 'мальтодекстрин', maker: 'Б', net_g: 500, price: 60, qty: 2 }
  assert.equal(cardTotal(a), 480)
  assert.equal(cardGrams(a), 5000)
  assert.equal(cardPerGram(a), 0.096)
  assert.equal(cardPerGram(b), 0.12)
  assert.equal(cardTitle(a), 'мальтодекстрин · А · 1000 г')
  assert.equal(cardTitle({ substance: 'фруктоза', net_g: 1000 }), 'фруктоза · 1000 г')
  const m = cardsByTx([a, b, { id: 'c', substance: 'x', net_g: 1, price: 1, qty: 1 }])
  assert.deepEqual(m.get('t1').map((c) => c.id), ['a', 'b'])
  assert.equal(m.size, 1) // карточка без операции в карту не попадает
  assert.deepEqual(cardSplit(-720, [a, b]), { sum: 600, diff: 120 })
  assert.deepEqual(cardSplit(-500, [a]), { sum: 480, diff: 20 })
  assert.deepEqual(cardSplit(-400, [a]), { sum: 480, diff: -80 })
})

test('подсказки: уникальные вещества и производители по алфавиту, пустые не берутся', () => {
  const l = [{ substance: 'фруктоза', maker: '' }, { substance: 'мальтодекстрин', maker: 'Б' }, { substance: 'мальтодекстрин', maker: 'А' }]
  assert.deepEqual(cardNames(l, 'substance'), ['мальтодекстрин', 'фруктоза'])
  assert.deepEqual(cardNames(l, 'maker'), ['А', 'Б'])
})

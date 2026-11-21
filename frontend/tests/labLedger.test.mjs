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

import { stockModel } from '../src/utils/labLedger.js'
test('stockModel: группы по веществу без учёта регистра, остаток от номинала, цены карточек раздельно', () => {
  const cards = [
    { id: 'a', day: '2026-11-10', substance: 'Мальтодекстрин', maker: 'А', net_g: 1000, price: 96, qty: 5 },
    { id: 'b', day: '2026-11-12', substance: 'мальтодекстрин', maker: 'Б', net_g: 500, price: 60, qty: 2 },
    { id: 'c', day: '2026-11-11', substance: 'фруктоза', maker: '', net_g: 1000, price: 120, qty: 2 }
  ]
  const m = stockModel(cards)
  assert.deepEqual(m.groups.map((g) => g.name), ['Мальтодекстрин', 'фруктоза'])
  assert.equal(m.groups[0].nominal, 6000)
  assert.equal(m.groups[0].left, 6000)
  assert.equal(m.groups[0].valueTotal, 600) // 480 + 2×60
  assert.equal(m.valueTotal, 840)
  assert.equal(m.valueLeft, 840)
  // расход по карточке a: 2500 г из 5000 — остаток по её цене 0,096 ₴/г
  const u = stockModel(cards, new Map([['a', 2500], ['c', 99999]]))
  assert.equal(u.groups[0].rows[0].left, 2500)
  assert.equal(u.groups[0].rows[0].pct, 0.5)
  assert.equal(u.groups[0].rows[0].valueLeft, 240)
  assert.equal(u.groups[1].left, 0) // списано больше номинала — остаток не уходит в минус
  assert.equal(u.groups[0].left, 2500 + 1000)
  assert.deepEqual(stockModel([]), { groups: [], cards: 0, valueTotal: 0, valueLeft: 0 })
})

import { batchCode, cleanBatch, batchUsage, batchCost } from '../src/utils/labLedger.js'
test('batchCode: ГГММДД-N, счёт по партиям того же дня', () => {
  assert.equal(batchCode('2026-11-21', []), '261121-1')
  assert.equal(batchCode('2026-11-21', [{ code: '261121-1' }, { code: '261121-2' }, { code: '261120-1' }]), '261121-3')
})
test('cleanBatch: номер, дата, строки с карточкой и граммами', () => {
  const ok = { day: '2026-11-21', code: ' 261121-1 ', pieces: '10', note: '', lines: [{ card_id: 'a', grams: '20,5' }] }
  assert.deepEqual(cleanBatch(ok), { day: '2026-11-21', code: '261121-1', pieces: 10, note: '', lines: [{ card_id: 'a', grams: 20.5 }] })
  assert.equal(cleanBatch({ ...ok, pieces: '' }).pieces, 0)
  assert.equal(cleanBatch({ ...ok, code: '' }), null)
  assert.equal(cleanBatch({ ...ok, day: '21.11.2026' }), null)
  assert.equal(cleanBatch({ ...ok, lines: [] }), null)
  assert.equal(cleanBatch({ ...ok, lines: [{ card_id: '', grams: 5 }] }), null)
  assert.equal(cleanBatch({ ...ok, lines: [{ card_id: 'a', grams: 0 }] }), null)
  assert.equal(cleanBatch({ ...ok, pieces: '2.5' }), null)
})
test('себестоимость партии: каждая строка по цене грамма своей карточки, расход считается по партиям', () => {
  const cards = [
    { id: 'a', substance: 'мальтодекстрин', net_g: 1000, price: 96, qty: 5 },
    { id: 'b', substance: 'мальтодекстрин', net_g: 500, price: 60, qty: 2 }
  ]
  const b = { id: 'p1', pieces: 10, lines: [{ card_id: 'a', grams: 100 }, { card_id: 'b', grams: 50 }, { card_id: 'gone', grams: 5 }] }
  const c = batchCost(b, cards)
  assert.deepEqual(c.lines.map((l) => l.cost), [9.6, 6, 0]) // 100×0,096; 50×0,12
  assert.equal(c.lines[2].missing, true)
  assert.equal(c.cost, 15.6)
  assert.equal(c.perPiece, 1.56)
  assert.equal(c.gramsPerPiece, 15.5)
  assert.equal(batchCost({ pieces: 0, lines: [{ card_id: 'a', grams: 1 }] }, cards).perPiece, null)
  const u = batchUsage([b, { id: 'p2', lines: [{ card_id: 'a', grams: 50 }] }])
  assert.equal(u.get('a'), 150)
  assert.equal(batchUsage([b], 'p1').size, 0) // править партию: её строки не считаются
})

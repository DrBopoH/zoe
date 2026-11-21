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

test('граммы — до миллиграмма: 0,125 г не округляется до 0,13, суммы без хвостов float', () => {
  const cards = [{ id: 'a', substance: 'x', net_g: 100, price: 135, qty: 1 }]
  assert.equal(cleanBatch({ day: '2026-11-21', code: '1', lines: [{ card_id: 'a', grams: '0,125' }] }).lines[0].grams, 0.125)
  assert.equal(cleanBatch({ day: '2026-11-21', code: '1', lines: [{ card_id: 'a', grams: '0.0004' }] }), null) // меньше миллиграмма — ноль
  assert.equal(batchUsage([{ id: 'p', lines: [{ card_id: 'a', grams: 0.1 }] }, { id: 'q', lines: [{ card_id: 'a', grams: 0.2 }] }]).get('a'), 0.3)
  const c = batchCost({ pieces: 3, lines: [{ card_id: 'a', grams: 0.125 }, { card_id: 'a', grams: 10 }] }, cards)
  assert.equal(c.grams, 10.125)
  assert.equal(c.gramsPerPiece, 3.375)
  assert.equal(stockModel(cards, new Map([['a', 0.3]])).groups[0].rows[0].left, 99.7)
})

import { cleanMove, batchPieces, moveAmounts } from '../src/utils/labLedger.js'
test('cleanMove: продажа с ценой, сжигание с делом, целые штуки', () => {
  const sale = { type: 'sale', batch_id: 'b', day: '2026-11-21', pieces: '3', price: '15,5', note: ' ' }
  assert.deepEqual(cleanMove(sale), { type: 'sale', batch_id: 'b', day: '2026-11-21', pieces: 3, note: '', price: 15.5 })
  assert.equal(cleanMove({ ...sale, price: '' }), null)
  assert.equal(cleanMove({ ...sale, price: '0' }).price, 0) // подарок по нулевой цене — всё равно продажа
  assert.equal(cleanMove({ ...sale, pieces: '0' }), null)
  assert.equal(cleanMove({ ...sale, pieces: '1.5' }), null)
  assert.equal(cleanMove({ ...sale, day: '' }), null)
  assert.equal(cleanMove({ ...sale, type: 'gift' }), null)
  const burn = { type: 'burn', batch_id: 'b', day: '2026-11-21', pieces: 2, target_ws: 'ws1', target_name: 'Glovo' }
  assert.deepEqual(cleanMove(burn), { type: 'burn', batch_id: 'b', day: '2026-11-21', pieces: 2, note: '', target_ws: 'ws1', target_name: 'Glovo' })
  assert.equal(cleanMove({ ...burn, target_ws: '' }), null)
})
test('штуки партии и деньги движений: сжигание по себестоимости без наценки, продажа с наценкой', () => {
  const cards = [{ id: 'a', substance: 'x', net_g: 1000, price: 96, qty: 5 }]
  const batch = { id: 'p', pieces: 10, lines: [{ card_id: 'a', grams: 200 }] } // 19,2 ₴ → 1,92 за штуку
  const moves = [{ batch_id: 'p', type: 'sale', pieces: 4, price: 15 }, { batch_id: 'p', type: 'burn', pieces: 2 }, { batch_id: 'other', pieces: 9 }]
  assert.deepEqual(batchPieces(batch, moves), { total: 10, moved: 6, left: 4 })
  assert.deepEqual(batchPieces({ id: 'p', pieces: 0 }, moves), { total: 0, moved: 6, left: 0 })
  assert.deepEqual(moveAmounts(moves[0], batch, cards), { cost: 7.68, revenue: 60, margin: 52.32 })
  assert.deepEqual(moveAmounts(moves[1], batch, cards), { cost: 3.84, revenue: 0, margin: 0 })
})

import { writeoffAmount, mergeUsage, writeoffUsage } from '../src/utils/labLedger.js'
test('списание остатка: убыток по цене грамма карточки, расход складывается из партий и списаний', () => {
  const card = { id: 'a', substance: 'x', net_g: 1000, price: 96, qty: 5 }
  assert.equal(writeoffAmount({ card_id: 'a', grams: 100 }, card), 9.6)
  assert.equal(writeoffAmount({ card_id: 'gone', grams: 100 }, undefined), 0)
  const u = mergeUsage(new Map([['a', 4900.1]]), writeoffUsage([{ card_id: 'a', grams: 99.9 }, { card_id: 'b', grams: 1 }]))
  assert.equal(u.get('a'), 5000)
  assert.equal(u.get('b'), 1)
  assert.equal(stockModel([card], u).groups[0].left, 0)
})

import { labEstimate } from '../src/utils/labLedger.js'
test('Смета лаба: деньги отдельно от склада, закуплено = осталось + в партиях + списано', () => {
  const cards = [{ id: 'a', day: '2026-11-10', substance: 'x', net_g: 1000, price: 96, qty: 5 }] // 480 ₴, 0,096 ₴/г
  const batches = [{ id: 'p', day: '2026-11-15', code: '261115-1', pieces: 10, lines: [{ card_id: 'a', grams: 200 }] }] // 19,2 ₴, 1,92 за штуку
  const moves = [
    { id: 'm1', batch_id: 'p', type: 'sale', pieces: 4, price: 15 },
    { id: 'm2', batch_id: 'p', type: 'burn', pieces: 2 },
    { id: 'm3', batch_id: 'gone', type: 'sale', pieces: 9, price: 1 }
  ]
  const writeoffs = [{ id: 'w', card_id: 'a', grams: 100 }] // 9,6 ₴
  const tx = [
    { id: 't1', time: '2026-11-10T10:00:00', amount: -500 },
    { id: 't2', time: '2026-11-11T10:00:00', amount: -40 },
    { id: 't3', time: '2026-11-16T10:00:00', amount: 50 }
  ]
  const marks = new Map([['t1', 'expense'], ['t2', 'loss'], ['t3', 'sale']])
  const entries = [{ id: 'e1', day: '2026-11-12', badge: 'expense', amount: 10 }, { id: 'e2', day: '2026-11-20', badge: 'loss', amount: 9.6, src: 'w' }]
  const m = labEstimate({ transactions: tx, marks, entries, cards, batches, moves, writeoffs })
  assert.equal(m.bank.expense.sum, -510) // 500 + наличные 10
  assert.equal(m.bank.loss.sum, -40) // убыток от списания — не деньги
  assert.equal(m.cash, -500) // 50 − 510 − 40
  assert.deepEqual(m.stock, { purchased: 480, left: 451.2, inBatches: 19.2, writtenOff: 9.6, gap: 0, cards: 1, substances: 1 })
  const b = m.batches[0]
  assert.deepEqual([b.unit, b.soldPieces, b.revenue, b.soldCost, b.margin, b.burnedPieces, b.compensation, b.left, b.leftValue], [1.92, 4, 60, 7.68, 52.32, 2, 3.84, 4, 7.68])
  assert.deepEqual(m.sales, { pieces: 4, revenue: 60, cost: 7.68, margin: 52.32 })
  assert.deepEqual(m.burns, { pieces: 2, compensation: 3.84 })
  assert.equal(m.finished, 7.68)
  assert.deepEqual(m.recon, { sold: 60, received: 50, diff: -10 })
  assert.equal(m.equity, r2eq(-500 + 3.84 + 451.2 + 7.68))
  const empty = labEstimate({})
  assert.equal(empty.cash, 0)
  assert.equal(empty.batches.length, 0)
})
const r2eq = (v) => Math.round(v * 100) / 100

import test from 'node:test'
import assert from 'node:assert/strict'
import { stripTag, groupAccounts, groupBadges, bankKeyOf, accountLook } from '../src/utils/statements.js'

test('stripTag: хвост-тег срезается только если он равен типу счёта', () => {
  assert.equal(stripTag('Volunets Taras.pdf (privat)', 'privat'), 'Volunets Taras.pdf')
  assert.equal(stripTag('Волинець Тарас (black)', 'BLACK'), 'Волинець Тарас')
  assert.equal(stripTag('Отчёт (март)', 'privat'), 'Отчёт (март)')
  assert.equal(stripTag('Без тега', 'privat'), 'Без тега')
})

test('groupAccounts: один файл — одна группа, группы по имени', () => {
  const g = groupAccounts([
    { id: '1', name: 'B.pdf (privat)', type: 'privat' },
    { id: '2', name: 'A.pdf (raif)', type: 'raif' },
    { id: '3', name: 'B.pdf (raif)', type: 'raif' },
    { id: '4', name: 'B.pdf (privat)', type: 'privat' }
  ])
  assert.deepEqual(g.map(x => x.name), ['A.pdf', 'B.pdf'])
  assert.equal(g[1].accounts.length, 3)
})

test('bankKeyOf / accountLook: у каждого счёта значок и цвет банка', () => {
  assert.equal(bankKeyOf({ type: 'binance' }), 'binance')
  assert.equal(accountLook({ type: 'binance' }).tag, 'HOT')
  assert.equal(accountLook({ type: 'icu' }).tag, 'OVDP')
  const p = accountLook({ type: 'privat' })
  assert.ok(p.tag)
  assert.match(p.color, /^#|^rgb|^hsl/)
})

test('groupBadges: значки группы без повторов', () => {
  const g = { accounts: [{ type: 'privat' }, { type: 'privat' }, { type: 'binance' }] }
  const tags = groupBadges(g).map(b => b.tag)
  assert.equal(tags.length, 2)
  assert.ok(tags.includes('HOT'))
})

import { summaryLayers } from '../src/utils/statements.js'

test('summaryLayers: общая ось по всем счетам, прозрачность 1/n, штриховка отдельно', () => {
  const a = { id: 'a', type: 'privat', axis_from: 0, axis_to: 100, cells: '1122' }
  const b = { id: 'b', type: 'raif', axis_from: 50, axis_to: 100, cells: '11' }
  const all = summaryLayers([a, b], [a, b], 100)
  assert.equal(all.from, 0)
  assert.equal(all.to, 100)
  assert.equal(all.fills.length, 2)
  assert.equal(all.hatches.length, 1)
  assert.equal(all.fills[0].alpha, 0.5)
  assert.equal(all.fills[0].width, 50)
  assert.equal(all.fills[1].left, 50) // b начинается с середины общей оси
  // выбор одного счёта ось не двигает
  const one = summaryLayers([a, b], [b], 100)
  assert.equal(one.from, 0)
  assert.equal(one.fills[0].alpha, 1)
  assert.equal(one.fills[0].left, 50)
})

test('summaryLayers: ось тянется до now, пустой ввод безопасен', () => {
  const a = { id: 'a', type: 'privat', axis_from: 0, axis_to: 50, cells: '11' }
  assert.equal(summaryLayers([a], [a], 100).fills[0].width, 50)
  assert.deepEqual(summaryLayers([], [], 5), { from: 0, to: 0, fills: [], hatches: [] })
})

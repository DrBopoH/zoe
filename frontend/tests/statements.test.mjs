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

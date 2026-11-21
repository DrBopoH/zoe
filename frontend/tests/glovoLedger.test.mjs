import test from 'node:test'
import assert from 'node:assert/strict'
import { BADGES, badgeOf, badgeTotals, weekMonday, weekSunday, nominalTotal } from '../src/utils/glovoLedger.js'

test('бейджи: пять штук, ключи латиницей (как требует бэкенд), стороны приход/расход', () => {
  assert.deepEqual(BADGES.map((b) => b.key), ['expense', 'loss', 'tips', 'income', 'payout'])
  for (const b of BADGES) assert.match(b.key, /^[a-z][a-z0-9-]{0,15}$/)
  assert.deepEqual(BADGES.filter((b) => b.side === 'out').map((b) => b.key), ['expense', 'loss'])
  assert.equal(badgeOf('tips').label, 'Чай')
  assert.equal(badgeOf('нет'), null)
})

test('итоги по бейджам: число и сумма, без копеечного хвоста; чужие и непомеченные не считаются', () => {
  const tx = [
    { id: 'a', amount: 0.1 }, { id: 'b', amount: 0.2 }, { id: 'c', amount: -450.5 }, { id: 'd', amount: 999 }
  ]
  const marks = new Map([['a', 'tips'], ['b', 'tips'], ['c', 'expense'], ['zzz', 'income'], ['d', 'неизвестный']])
  const r = badgeTotals(tx, marks)
  assert.deepEqual(r.tips, { count: 2, sum: 0.3 })
  assert.deepEqual(r.expense, { count: 1, sum: -450.5 })
  assert.deepEqual(r.income, { count: 0, sum: 0 })
  assert.equal(Object.keys(r).length, BADGES.length)
})

test('понедельник недели по любой дате; мусор и несуществующие даты — пусто', () => {
  assert.equal(weekMonday('2026-11-16'), '2026-11-16') // понедельник
  assert.equal(weekMonday('2026-11-22'), '2026-11-16') // воскресенье
  assert.equal(weekMonday('2026-11-21'), '2026-11-16')
  assert.equal(weekMonday('2026-03-01'), '2026-02-23') // через границу месяца
  assert.equal(weekMonday('2026-01-01'), '2025-12-29') // через границу года
  assert.equal(weekMonday('2026-02-30'), '')
  assert.equal(weekMonday('вчера'), '')
  assert.equal(weekMonday(''), '')
})

test('воскресенье недели и сумма номинала', () => {
  assert.equal(weekSunday('2026-11-16'), '2026-11-22')
  assert.equal(weekSunday('2026-12-28'), '2027-01-03')
  assert.equal(nominalTotal([{ nominal: 1500.1 }, { nominal: 700.2 }]), 2200.3)
  assert.equal(nominalTotal([]), 0)
})

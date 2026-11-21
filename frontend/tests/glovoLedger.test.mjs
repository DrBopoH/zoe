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

import { timelineWeeks, timelineSummary, txDate, addDays, PAYOUT_BADGES } from '../src/utils/glovoLedger.js'

// время без «Z»: разбирается в локальном поясе, дата операции не уезжает на соседний день
const tx = (id, date, amount) => ({ id, time: `${date}T12:00:00`, amount })

test('addDays и txDate: через границу месяца, локальная дата', () => {
  assert.equal(addDays('2026-11-30', 1), '2026-12-01')
  assert.equal(addDays('2026-01-01', -1), '2025-12-31')
  assert.equal(txDate({ time: '2026-11-18T12:00:00' }), '2026-11-18')
  assert.equal(txDate({ time: 'мусор' }), '')
  assert.deepEqual(PAYOUT_BADGES, ['income', 'payout'])
})

test('таймлайн: номинал недели N сверяется с приходом недели N+1 (через 2–4 дня после отчёта)', () => {
  const rows = timelineWeeks({
    weeks: [{ week: '2026-11-09', nominal: 2200 }],
    transactions: [tx('a', '2026-11-18', 1500), tx('b', '2026-11-20', 400), tx('c', '2026-11-25', 999), tx('t', '2026-11-19', 300)],
    marks: new Map([['a', 'income'], ['b', 'payout'], ['t', 'tips'], ['c', 'income']]),
    today: '2026-11-21'
  })
  assert.equal(rows.length, 12) // лента не короче 12 недель
  assert.equal(rows.at(-1).week, '2026-11-16')
  const w = rows.find((r) => r.week === '2026-11-09')
  assert.equal(w.nominal, 2200)
  assert.equal(w.received, 1900) // 1500 + 400 в неделю 16–22.11; чай и чужая неделя не считаются
  assert.equal(w.count, 2)
  assert.equal(w.firstDate, '2026-11-18')
  assert.equal(w.lagDays, 3) // с воскресенья 15.11 до среды 18.11
  assert.equal(w.delta, -300)
  assert.equal(w.deltaPct, 13.64)
  assert.equal(w.status, 'paid')
})

test('статусы: ждём, пока идёт неделя прихода; «нет денег», когда она прошла; без номинала — none', () => {
  const base = { transactions: [], marks: new Map() }
  const pending = timelineWeeks({ ...base, weeks: [{ week: '2026-11-16', nominal: 1000 }], today: '2026-11-21' })
  assert.equal(pending.find((r) => r.week === '2026-11-16').status, 'pending')
  const missing = timelineWeeks({ ...base, weeks: [{ week: '2026-11-02', nominal: 1000 }], today: '2026-11-21' })
  assert.equal(missing.find((r) => r.week === '2026-11-02').status, 'missing') // неделя прихода 09–15.11 прошла
  assert.equal(missing.find((r) => r.week === '2026-11-09').status, 'none')
})

test('таймлайн начинается с прихода, у которого нет номинала; пусто без данных; доходит до текущей недели', () => {
  const rows = timelineWeeks({ weeks: [], transactions: [tx('a', '2026-11-04', 700)], marks: new Map([['a', 'payout']]), today: '2026-11-21', minWeeks: 1 })
  assert.equal(rows[0].week, '2026-10-26') // неделя прихода 02–08.11 → номинал за 26.10–01.11
  assert.equal(rows[0].received, 700)
  assert.equal(rows[0].status, 'none')
  assert.equal(rows[0].delta, null)
  assert.equal(rows.at(-1).week, '2026-11-16')
  // без данных всё равно есть куда вносить: последние 12 недель, и их можно продлить назад
  const empty = timelineWeeks({ weeks: [], transactions: [], marks: new Map(), today: '2026-11-21' })
  assert.equal(empty.length, 12)
  assert.equal(empty.at(-1).week, '2026-11-16')
  assert.equal(empty[0].week, '2026-08-31')
  assert.ok(empty.every((r) => r.status === 'none' && r.nominal === null))
  const older = timelineWeeks({ weeks: [], transactions: [], marks: new Map(), today: '2026-11-21', back: 8 })
  assert.equal(older.length, 20)
  assert.equal(older[0].week, '2026-07-06')
})

test('номинал в будущей неделе не рисуется после текущей, а старый хвост ограничен', () => {
  const rows = timelineWeeks({ weeks: [{ week: '2030-01-07', nominal: 1 }, { week: '2010-01-04', nominal: 1 }], transactions: [], marks: new Map(), today: '2026-11-21' })
  assert.equal(rows.at(-1).week, '2026-11-16')
  assert.ok(rows.length <= 261)
})

test('сводка: только недели с номиналом и пришедшими деньгами', () => {
  const rows = [
    { status: 'paid', nominal: 2200, received: 1900 },
    { status: 'paid', nominal: 1000, received: 800.5 },
    { status: 'pending', nominal: 3000, received: 0 },
    { status: 'none', nominal: null, received: 500 }
  ]
  assert.deepEqual(timelineSummary(rows), { weeks: 2, nominal: 3200, received: 2700.5, withheld: 499.5, withheldPct: 15.61 })
  assert.deepEqual(timelineSummary([]), { weeks: 0, nominal: 0, received: 0, withheld: 0, withheldPct: null })
})

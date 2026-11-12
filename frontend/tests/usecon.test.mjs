import test from 'node:test'
import assert from 'node:assert/strict'
import { sahmRule, technicalRecession, curveState, vixLevel, USECON_IDS, SERIES, unitText } from '../src/utils/macro.js'

const monthly = (vals) => vals.map((v, i) => ({ d: `${2025 + Math.floor(i / 12)}-${String((i % 12) + 1).padStart(2, '0')}-01`, v }))

test('правило Сам: безработица выросла на 0,5 п. п. над минимумом года — сигнал', () => {
  const calm = monthly(Array(18).fill(4))
  assert.deepEqual(sahmRule(calm), { value: 0, signal: false })
  const rising = monthly([...Array(15).fill(4), 4.4, 4.8, 5.2])
  const r = sahmRule(rising)
  assert.ok(r.signal && r.value >= 0.5, JSON.stringify(r))
  assert.equal(sahmRule(monthly([4, 4, 4])), null)
})

test('техническая рецессия — два квартала спада подряд', () => {
  assert.equal(technicalRecession([{ v: 2 }, { v: -0.5 }, { v: -0.1 }]), true)
  assert.equal(technicalRecession([{ v: -1 }, { v: 0.5 }]), false)
  assert.equal(technicalRecession([{ v: -1 }]), false)
})

test('кривая и VIX словами', () => {
  assert.equal(curveState(-0.2), 'inverted')
  assert.equal(curveState(0.2), 'flat')
  assert.equal(curveState(1.1), 'normal')
  assert.equal(curveState(null), null)
  assert.equal(vixLevel(15), 'calm')
  assert.equal(vixLevel(25), 'nervous')
  assert.equal(vixLevel(40), 'panic')
})

test('все ряды экономики США описаны в SERIES и читаются без ошибок', () => {
  for (const id of USECON_IDS) assert.ok(SERIES[id] && SERIES[id].label, id)
  assert.equal(unitText(18.5, 'п.'), '18,5 п.')
})

import { drawdown, athInfo, changeSince, fearGreedLevel, CRYPTO_IDS } from '../src/utils/macro.js'

test('просадка от максимума и позиция относительно пика', () => {
  const pts = [{ d: '2021-11-01', v: 60000 }, { d: '2022-06-01', v: 20000 }, { d: '2024-03-01', v: 70000 }, { d: '2026-11-01', v: 35000 }]
  assert.deepEqual(drawdown(pts).map((p) => Math.round(p.v)), [0, -67, 0, -50])
  const a = athInfo(pts)
  assert.equal(a.ath, 70000)
  assert.equal(a.date, '2024-03-01')
  assert.ok(Math.abs(a.fromAth + 50) < 1e-9)
  assert.equal(athInfo([]), null)
})

test('изменение за год и индекс страха словами', () => {
  const pts = [{ d: '2025-11-01', v: 50000 }, { d: '2026-11-01', v: 75000 }]
  assert.ok(Math.abs(changeSince(pts, '2026-11-12', 1) - 50) < 1e-9)
  assert.equal(changeSince(pts, '2026-11-12', 10), null)
  assert.equal(fearGreedLevel(10), 'extreme-fear')
  assert.equal(fearGreedLevel(30), 'fear')
  assert.equal(fearGreedLevel(50), 'neutral')
  assert.equal(fearGreedLevel(65), 'greed')
  assert.equal(fearGreedLevel(90), 'extreme-greed')
  assert.ok(CRYPTO_IDS.every((id) => id.startsWith('crypto_')))
})

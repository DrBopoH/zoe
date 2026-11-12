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

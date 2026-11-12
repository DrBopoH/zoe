import test from 'node:test'
import assert from 'node:assert/strict'
import { bennerKind, bennerPhase, bennerSpans } from '../src/utils/benner.js'
import { msToSlot, slotToMs } from '../src/utils/candleMath.js'

test('вид года по циклу Беннера; паника главнее пика', () => {
  assert.equal(bennerKind(2019), 'A')
  assert.equal(bennerKind(2026), 'B')
  assert.equal(bennerKind(2023), 'C')
  assert.equal(bennerKind(1999), 'A')
  assert.equal(bennerKind(2024), null)
})

test('фаза: ключевой год или период после предыдущего ключевого', () => {
  assert.deepEqual(bennerPhase(2026), { kind: 'B', key: true })
  assert.deepEqual(bennerPhase(2024), { kind: 'C', key: false }) // после дна 2023
  assert.deepEqual(bennerPhase(2008), { kind: 'B', key: false }) // после пика 2007
  assert.equal(bennerPhase(1700), null)
})

test('участки в окне: ключевые годы отдельно, годы между ними склеены', () => {
  const t = (y) => Date.UTC(y, 0, 1)
  const xOf = (ms) => 100 + ((ms - t(2020)) / (t(2030) - t(2020))) * 800
  const sp = bennerSpans({ fromMs: t(2020), toMs: t(2030), xOf, left: 100, right: 900 })
  const keys = sp.filter((s) => s.key).map((s) => [s.from, s.kind])
  assert.deepEqual(keys, [[2023, 'C'], [2026, 'B']])
  const gap = sp.find((s) => !s.key && s.from === 2024)
  assert.equal(gap.to, 2026) // 2024–2025 одним участком
  for (const s of sp) assert.ok(s.x >= 100 - 1e-9 && s.x + s.w <= 900 + 1e-9 && s.w > 0)
})

test('время ↔ слоты: обратимо внутри данных и за краями', () => {
  const starts = [0, 100, 200, 300]
  for (const s of [-1.5, 0, 0.5, 2.25, 3, 3.5, 6]) assert.ok(Math.abs(msToSlot(starts, slotToMs(starts, s)) - s) < 1e-9, String(s))
  assert.equal(msToSlot([], 5), null)
})

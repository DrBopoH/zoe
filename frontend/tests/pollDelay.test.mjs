import test from 'node:test'
import assert from 'node:assert/strict'
import { pollDelay, POLL_BUSY_MS, POLL_IDLE_MS, POLL_LOST_MS } from '../src/utils/pollDelay.js'

test('идёт синк или опрос рынка — часто, в простое — реже, без связи — снова часто', () => {
  assert.equal(pollDelay({ syncing: true }), POLL_BUSY_MS)
  assert.equal(pollDelay({ marketRunning: true }), POLL_BUSY_MS)
  assert.equal(pollDelay({}), POLL_IDLE_MS)
  assert.equal(pollDelay(), POLL_IDLE_MS)
  assert.equal(pollDelay({ lost: true }), POLL_LOST_MS)
  assert.ok(POLL_IDLE_MS > POLL_BUSY_MS)
})

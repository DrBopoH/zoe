import test from 'node:test'
import assert from 'node:assert/strict'
import { marketPanelOpen, feedView, elapsedText } from '../src/utils/marketPanel.js'

const run = (extra = {}) => ({ running: true, started_at: 10, feeds: [{ id: 'nbu', state: 'fetch' }], ...extra })

test('окно открыто только пока идёт опрос и запуск не закрыт крестиком', () => {
  assert.equal(marketPanelOpen(run(), null), true)
  assert.equal(marketPanelOpen(run(), 10), false) // этот запуск закрыт
  assert.equal(marketPanelOpen(run({ started_at: 11 }), 10), true) // новый запуск открывается снова
  assert.equal(marketPanelOpen(run({ running: false }), null), false)
  assert.equal(marketPanelOpen(run({ feeds: [] }), null), false)
  assert.equal(marketPanelOpen(null, null), false)
  assert.equal(marketPanelOpen({}, null), false) // сервер без поля market
})

test('строка источника: очередь, бегущая полоса, процент, готово, ошибка', () => {
  assert.deepEqual(feedView({ state: 'queued' }), { mode: 'queued', pct: 0, text: 'в очереди' })
  assert.deepEqual(feedView({ state: 'fetch' }), { mode: 'run', pct: 0, text: '' })
  assert.deepEqual(feedView({ state: 'fetch', pct: 42.4, note: 'аукционы: 10.11' }), { mode: 'pct', pct: 42, text: 'аукционы: 10.11' })
  assert.equal(feedView({ state: 'fetch', pct: 250 }).pct, 100)
  assert.deepEqual(feedView({ state: 'done', fetched: 184 }), { mode: 'done', pct: 100, text: 'Бумаг: 184' })
  assert.deepEqual(feedView({ state: 'error', error: 'HTTP 502' }), { mode: 'error', pct: 0, text: 'HTTP 502' })
  assert.equal(feedView({ state: 'error' }).text, 'ошибка источника')
})

test('прошедшее время м:сс', () => {
  assert.equal(elapsedText(100, 100_000), '0:00')
  assert.equal(elapsedText(100, 165_000), '1:05')
  assert.equal(elapsedText(100, 50_000), '0:00') // часы клиента отстали — не уходим в минус
})

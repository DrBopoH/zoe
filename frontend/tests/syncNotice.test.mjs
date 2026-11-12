import test from 'node:test'
import assert from 'node:assert/strict'
import { syncOkLines } from '../src/utils/syncNotice.js'

test('без автозаписи — только запросы', () => {
  assert.deepEqual(syncOkLines({ requests: 12 }), ['Запросов к банку: 12'])
  assert.deepEqual(syncOkLines({ requests: 0, autosave: false, saved: 0 }), ['Запросов к банку: 0'])
})

test('автозапись: сколько источников обновлено или что уже актуальны', () => {
  assert.deepEqual(syncOkLines({ requests: 3, autosave: true, saved: 2 }), ['Запросов к банку: 3', 'Записано в источники: 2'])
  assert.deepEqual(syncOkLines({ requests: 1, autosave: true, saved: 0 }), ['Запросов к банку: 1', 'Источники уже актуальны'])
})

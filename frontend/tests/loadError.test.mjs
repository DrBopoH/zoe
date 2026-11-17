import test from 'node:test'
import assert from 'node:assert/strict'
import { loadErrorText, firstLoadError } from '../src/utils/loadError.js'

test('сетевой сбой браузера — «нет связи», текст бэкенда остаётся как есть', () => {
  assert.equal(loadErrorText('Failed to fetch'), 'Нет связи с бэкендом')
  assert.equal(loadErrorText('NetworkError when attempting to fetch resource.'), 'Нет связи с бэкендом')
  assert.equal(loadErrorText('Load failed'), 'Нет связи с бэкендом')
  assert.equal(loadErrorText('GET /macro: 500'), 'GET /macro: 500')
  assert.equal(loadErrorText(''), '')
  assert.equal(loadErrorText(undefined), '')
})

test('из нескольких источников берётся первая непустая ошибка', () => {
  assert.equal(firstLoadError('', '', ''), '')
  assert.equal(firstLoadError('', 'Failed to fetch', 'другая'), 'Нет связи с бэкендом')
  assert.equal(firstLoadError(undefined, null, 'ошибка реестра'), 'ошибка реестра')
  assert.equal(firstLoadError(), '')
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveNav } from '../src/utils/nav.js'

test('новые значения читаются как есть', () => {
  assert.deepEqual(resolveNav('dashboard', 'history'), { tab: 'dashboard', section: 'history' })
  assert.deepEqual(resolveNav('data', 'persons'), { tab: 'data', section: 'persons' })
})

test('старые вкладки переезжают в «Данные»', () => {
  assert.deepEqual(resolveNav('accounts', null), { tab: 'data', section: 'history' })
  assert.deepEqual(resolveNav('entities', null), { tab: 'data', section: 'persons' })
  assert.deepEqual(resolveNav('entities', 'history').section, 'persons') // старая вкладка главнее сохранённого раздела
})

test('мусор и пустое → Смета и «История»', () => {
  assert.deepEqual(resolveNav(null, null), { tab: 'dashboard', section: 'history' })
  assert.deepEqual(resolveNav('x', 'y'), { tab: 'dashboard', section: 'history' })
  assert.equal(resolveNav('data', 'zzz').section, 'history')
})

test('вкладка «Рынок» сохраняется как есть', () => {
  assert.deepEqual(resolveNav('market', 'persons'), { tab: 'market', section: 'persons' })
})

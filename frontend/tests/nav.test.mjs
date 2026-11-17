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

test('мусор и пустое → главная и «История»', () => {
  assert.deepEqual(resolveNav(null, null), { tab: 'home', section: 'history' })
  assert.deepEqual(resolveNav('x', 'y'), { tab: 'home', section: 'history' })
  assert.equal(resolveNav('data', 'zzz').section, 'history')
})

test('вкладка «Рынок» сохраняется как есть', () => {
  assert.deepEqual(resolveNav('market', 'persons'), { tab: 'market', section: 'persons' })
})

test('ARCHIVE_PAGES: в архиве три прежние страницы, главная вне архива', async () => {
  const { ARCHIVE_PAGES, TOP_TABS, HOME_TAB } = await import('../src/utils/nav.js')
  assert.deepEqual(ARCHIVE_PAGES.map((p) => p.key), TOP_TABS.filter((k) => k !== HOME_TAB))
  assert.deepEqual(ARCHIVE_PAGES.map((p) => p.label), ['Смета', 'Рынок', 'Учет'])
  assert.equal(resolveNav('home', null).tab, 'home')
})

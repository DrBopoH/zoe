import test from 'node:test'
import assert from 'node:assert/strict'
import { WORKSPACE_PAGES, pageOf } from '../src/utils/workspacePages.js'

test('реестр: ключи уникальны, латиницей строчными, как требует бэкенд', () => {
  const keys = WORKSPACE_PAGES.map((p) => p.key)
  assert.equal(new Set(keys).size, keys.length)
  for (const k of keys) assert.match(k, /^[a-z0-9][a-z0-9-]{0,31}$/)
  for (const p of WORKSPACE_PAGES) assert.equal(typeof p.load, 'function')
})

test('pageOf: известный ключ — запись, пустой и незнакомый — null', () => {
  assert.equal(pageOf('glovo')?.key, 'glovo')
  assert.equal(pageOf(''), null)
  assert.equal(pageOf(undefined), null)
  assert.equal(pageOf('нет-такой'), null)
})

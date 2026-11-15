import test from 'node:test'
import assert from 'node:assert/strict'
import { WORLD_INDICES, WORLD_GROUPS, worldIds, worldRows, buffettLevel } from '../src/utils/worldmarkets.js'
import { SERIES } from '../src/utils/macro.js'
import { MARKET_SECTIONS, feedOf } from '../src/utils/marketNav.js'

test('группы покрывают все индексы, у каждого ряда есть подпись и цвет', () => {
  const all = WORLD_GROUPS.flatMap((g) => worldIds(g.key)).sort()
  assert.deepEqual(all, WORLD_INDICES.map((i) => i.id).sort())
  for (const id of [...all, 'val_buffett', 'val_cape', 'val_divy']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('строки: последняя точка, изменение за год, удаление от максимума; нет данных — null', () => {
  const series = { wm_us: [{ d: '2025-05-01', v: 100 }, { d: '2026-03-01', v: 150 }, { d: '2026-05-01', v: 120 }] }
  const rows = worldRows(series, '2026-06-01')
  const us = rows.find((r) => r.id === 'wm_us')
  assert.equal(us.last.v, 120)
  assert.ok(Math.abs(us.chg1y - 20) < 1e-9)
  assert.ok(Math.abs(us.fromAth - -20) < 1e-9)
  const jp = rows.find((r) => r.id === 'wm_jp')
  assert.equal(jp.last, null)
  assert.equal(jp.chg1y, null)
  assert.equal(jp.fromAth, null)
})

test('индекс Баффета словами', () => {
  assert.equal(buffettLevel(80), 'дёшево')
  assert.equal(buffettLevel(100), 'умеренно')
  assert.equal(buffettLevel(140), 'дорого')
  assert.equal(buffettLevel(250), 'очень дорого')
  assert.equal(buffettLevel(null), null)
})

test('раздел «Мировые акции» в навигации читает источник world', () => {
  assert.ok(MARKET_SECTIONS.includes('world'))
  assert.equal(feedOf('world'), 'world')
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { ref } from 'vue'
import { feedStamps } from '../src/utils/feedStamps.js'

const feed = () => { const f = { series: ref({}), loads: 0, load() { f.loads++; f.series.value = { a: [{ d: '2026-01-01', v: 1 }] } } }; return f }

test('пустой фид грузится, свежий — нет', () => {
  let ver = 1
  const s = feedStamps(() => ver)
  const f = feed()
  s.ensure(f); s.ensure(f); s.ensure(f)
  assert.equal(f.loads, 1)
})

test('после записи в кеш (dataVersion) раздел, открытый раньше, перечитывается при возврате', () => {
  let ver = 1
  const s = feedStamps(() => ver)
  const a = feed(), b = feed()
  s.load(a)                  // открыт раздел A
  ver = 2; s.load(b)         // кеш обновился, пока был открыт B: перечитался только B
  assert.equal(a.loads, 1)
  s.ensure(a)                // возврат в A
  assert.equal(a.loads, 2)
  s.ensure(a)
  assert.equal(a.loads, 2)
  assert.equal(s.stale(b), false)
})

test('сбой загрузки (ряды пусты) не считается свежим', () => {
  const s = feedStamps(() => 1)
  const f = { series: ref({}), loads: 0, load() { f.loads++ } }
  s.load(f); s.ensure(f)
  assert.equal(f.loads, 2)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { scrollMode, scrollTarget } from '../src/utils/stackScroll.js'

test('режим: не переполнено или у начала — top, у конца — bottom, посередине — free', () => {
  assert.equal(scrollMode(0, 300, 300), 'top')
  assert.equal(scrollMode(0, 800, 300), 'top')
  assert.equal(scrollMode(1, 800, 300), 'top') // допуск в пару пикселей
  assert.equal(scrollMode(500, 800, 300), 'bottom')
  assert.equal(scrollMode(499, 800, 300), 'bottom')
  assert.equal(scrollMode(200, 800, 300), 'free')
})

test('цель прокрутки после смены высоты', () => {
  assert.equal(scrollTarget('top', 1000, 300), 0)
  assert.equal(scrollTarget('bottom', 1000, 300), 700)
  assert.equal(scrollTarget('bottom', 200, 300), 0)
  assert.equal(scrollTarget('free', 1000, 300), null)
})

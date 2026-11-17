import test from 'node:test'
import assert from 'node:assert/strict'
import { errText } from '../src/utils/errText.js'
import { ruFormat, intText } from '../src/utils/numfmt.js'
import { formatMoney, fmtN } from '../src/utils/formatters.js'

test('errText: сообщение Error, иначе сам выброшенный объект', () => {
  assert.equal(errText(new Error('нет связи')), 'нет связи')
  assert.equal(errText('просто строка'), 'просто строка')
  assert.equal(errText({ message: 'из объекта' }), 'из объекта')
  assert.equal(errText(new Error('')), 'Error') // пустое сообщение: хоть имя ошибки, а не пустая плашка
  assert.equal(errText(undefined), 'undefined')
  assert.equal(errText(null), 'null')
})

test('общие форматтеры Intl дают то же, что отдельный new Intl.NumberFormat', () => {
  const ru = new Intl.NumberFormat('ru-RU'), uk = new Intl.NumberFormat('uk-UA', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  for (const n of [0, 1, 999, 1234.5, 1234567.891, -42.5, 1e9]) {
    assert.equal(ruFormat(n), ru.format(n))
    assert.equal(formatMoney(n), uk.format(n))
    assert.equal(fmtN(n), ru.format(n))
    assert.equal(intText(n), ru.format(Math.round(n)))
  }
  assert.equal(formatMoney(null), uk.format(0))
  assert.equal(fmtN(undefined), ru.format(0))
})

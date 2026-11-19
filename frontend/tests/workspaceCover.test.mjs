import test from 'node:test'
import assert from 'node:assert/strict'
import { PALETTE, NAME_MAX, FALLBACK_COLOR, cleanName, nameOk, validColor, colorFor, coverStyle, fitSize } from '../src/utils/workspaceCover.js'

test('имя: пробелы схлопываются, пустое и слишком длинное не проходят', () => {
  assert.equal(cleanName('  Glovo   курьер '), 'Glovo курьер')
  assert.equal(nameOk('   '), false)
  assert.equal(nameOk('Glovo'), true)
  assert.equal(nameOk('я'.repeat(NAME_MAX)), true)
  assert.equal(nameOk('я'.repeat(NAME_MAX + 1)), false)
})

test('цвет: только #rrggbb, регистр приводится', () => {
  assert.equal(validColor('#ABCDEF'), '#abcdef')
  assert.equal(validColor('red'), '')
  assert.equal(validColor('#fff'), '')
})

test('colorFor стабилен и берёт из палитры', () => {
  assert.equal(colorFor('Glovo'), colorFor('  Glovo '))
  assert.ok(PALETTE.includes(colorFor('Семейный бюджет')))
})

test('coverStyle: картинка поверх цвета; без картинки — цвет и мягкий градиент; мусор → запасной цвет', () => {
  const img = coverStyle({ color: '#10b981', image: 'data:image/png;base64,AAAA' })
  assert.equal(img.backgroundColor, '#10b981')
  assert.match(img.backgroundImage, /^url\("data:image\/png/)
  const flat = coverStyle({ color: '#10b981', image: '' })
  assert.match(flat.backgroundImage, /linear-gradient/)
  assert.equal(coverStyle({ color: 'x', image: 'http://evil' }).backgroundColor, FALLBACK_COLOR)
  assert.doesNotMatch(coverStyle({ image: 'http://evil' }).backgroundImage, /evil/)
})

test('fitSize: уменьшает по длинной стороне, не растягивает', () => {
  assert.deepEqual(fitSize(1280, 640, 640), { w: 640, h: 320 })
  assert.deepEqual(fitSize(100, 50, 640), { w: 100, h: 50 })
})

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { MARKET_TREE, MARKET_SECTIONS, leafKeys, groupOf, feedOf, isBondSection, isMacroSection } from '../src/utils/marketNav.js'
import { macroWindow, RANGES } from '../src/utils/macro.js'

test('дерево рынка: листья и группы', () => {
  assert.deepEqual(MARKET_SECTIONS, ['nbu', 'ust', 'rates', 'inflation', 'fx'])
  assert.deepEqual(leafKeys(MARKET_TREE), MARKET_SECTIONS)
  assert.equal(groupOf(MARKET_TREE, 'inflation'), 'macro')
  assert.equal(groupOf(MARKET_TREE, 'ust'), 'securities')
  assert.equal(groupOf(MARKET_TREE, 'nope'), null)
  assert.ok(isBondSection('nbu') && !isBondSection('rates'))
  assert.ok(isMacroSection('rates') && !isMacroSection('ust'))
})

test('источник для «Обновить»: бумаги свой, макро общий', () => {
  assert.equal(feedOf('nbu'), 'nbu')
  assert.equal(feedOf('ust'), 'ust')
  assert.equal(feedOf('rates'), 'macro')
  assert.equal(feedOf('inflation'), 'macro')
  assert.equal(feedOf('fx'), 'fx')
})

test('окно «С 2007» начинается с 2007-01-01, но не левее данных', () => {
  const series = { cpi_yoy: [{ d: '2007-01-01', v: 10 }, { d: '2026-10-01', v: 9 }] }
  const r = RANGES.find((x) => x.key === '2007')
  const w = macroWindow({ series, ids: ['cpi_yoy'], todayIso: '2026-11-12', from: r.from })
  assert.ok(Math.abs(w.x0 + 19.86) < 0.05, String(w.x0))
  const late = macroWindow({ series: { cpi_yoy: [{ d: '2015-01-01', v: 1 }] }, ids: ['cpi_yoy'], todayIso: '2026-11-12', from: '2007-01-01' })
  assert.ok(late.x0 > -12)
})

// Каждый @click="имя" в шаблоне должен быть объявлен в script setup (потерянный refresh делал кнопку мёртвой)
test('обработчики шаблонов объявлены', () => {
  for (const f of ['MarketView', 'BondsView', 'MacroRatesView', 'MacroInflationView', 'MacroFxView', 'MacroPanel']) {
    const src = readFileSync(new URL(`../src/${f === 'MacroPanel' ? 'components' : 'views'}/${f}.vue`, import.meta.url), 'utf8')
    const script = src.slice(0, src.indexOf('</script>'))
    for (const m of src.matchAll(/@click="([A-Za-z_$][\w$]*)"/g)) {
      assert.match(script, new RegExp(`(const|function|let)\\s+(\\{[^}]*\\b${m[1]}\\b[^}]*\\}|${m[1]})\\b|\\b${m[1]}\\b\\s*[,}]`), `${f}: ${m[1]} не объявлен`)
    }
  }
})

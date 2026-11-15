import test from 'node:test'
import assert from 'node:assert/strict'
import { COUNTRY_METRICS, countryIds, countryRows, memberIds, spreadIds } from '../src/utils/countries.js'
import { SERIES } from '../src/utils/macro.js'

const ez = { cc: 'ez', name: 'Еврозона', currency: 'EUR', metrics: { policy: 'ez_policy', cpi: 'eu_cpi_yoy', y10: 'ez_y10', fx: 'eur_uah' } }
const ua = { cc: 'ua', name: 'Украина', currency: 'UAH', metrics: { policy: 'policy_rate', cpi: 'cpi_yoy' } }

test('у каждого ряда еврозоны есть подпись и цвет', () => {
  for (const id of ['ez_policy', 'ez_mro', 'ez_estr', 'ez_y2', 'ez_y10', 'ez_cpi_core', 'ez_u', 'ez_gdp']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('id рядов для карточек: без повторов', () => {
  assert.deepEqual(countryIds([ez, { ...ua, metrics: { policy: 'ez_policy', cpi: 'cpi_yoy' } }]).sort(), ['cpi_yoy', 'eu_cpi_yoy', 'eur_uah', 'ez_policy', 'ez_y10'])
  assert.deepEqual(countryIds(null), [])
})

test('строки карточки: порядок показателей, последние значения, нет данных — прочерк', () => {
  const series = { ez_policy: [{ d: '2026-09-01', v: 2 }, { d: '2026-10-03', v: 1.75 }], eur_uah: [{ d: '2026-11-10', v: 49.5 }] }
  const rows = countryRows(ez, series)
  assert.deepEqual(rows.map((r) => r.key), ['policy', 'cpi', 'y10', 'fx'])
  assert.equal(rows[0].text, '1,75 %')
  assert.equal(rows[0].last.d, '2026-10-03')
  assert.equal(rows[1].text, '—')
  assert.equal(rows[1].last, null)
  assert.equal(rows[3].text, '49,5 ₴')
})

test('страна без показателя не получает строку; пустой ввод не падает', () => {
  assert.deepEqual(countryRows(ua, {}).map((r) => r.key), ['policy', 'cpi'])
  assert.deepEqual(countryRows(null, null), [])
  assert.equal(COUNTRY_METRICS.length, new Set(COUNTRY_METRICS.map((m) => m.key)).size)
})

test('ряды стран еврозоны: у каждого есть подпись и цвет, у спредов — тоже', () => {
  assert.deepEqual(memberIds('y10'), ['de_y10', 'fr_y10', 'it_y10', 'es_y10', 'nl_y10'])
  assert.deepEqual(spreadIds(), ['it_bund_spread', 'fr_bund_spread', 'es_bund_spread'])
  for (const id of [...memberIds('cpi'), ...memberIds('u'), ...memberIds('y10'), ...spreadIds()]) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

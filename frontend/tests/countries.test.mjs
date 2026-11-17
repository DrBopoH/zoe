import test from 'node:test'
import assert from 'node:assert/strict'
import { COUNTRY_METRICS, COUNTRY_PANELS, countryIds, countryRows, memberIds, spreadIds } from '../src/utils/countries.js'
import { SERIES } from '../src/utils/macro.js'

const ez = { cc: 'ez', name: 'Еврозона', currency: 'EUR', metrics: { policy: 'ez_policy', cpi: 'eu_cpi_yoy', y10: 'ez_y10', fx: 'eur_uah' } }
const ua = { cc: 'ua', name: 'Украина', currency: 'UAH', metrics: { policy: 'policy_rate', cpi: 'cpi_yoy' } }

test('у каждого ряда еврозоны есть подпись и цвет', () => {
  for (const id of ['ez_policy', 'ez_mro', 'ez_estr', 'ez_y2', 'ez_y10', 'ez_cpi_core', 'ez_u', 'ez_gdp']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('id рядов для карточек: без повторов', () => {
  assert.deepEqual(countryIds([ez, { ...ua, cc: 'zz', metrics: { policy: 'ez_policy', cpi: 'cpi_yoy' } }]).sort(), ['cpi_yoy', 'eu_cpi_yoy', 'eur_uah', 'ez_policy', 'ez_y10'])
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

test('у рядов Великобритании и Швейцарии есть подпись и цвет', () => {
  for (const id of ['gb_policy', 'gb_sonia', 'gb_y10', 'gb_cpi', 'gb_u', 'gb_gdp', 'ch_policy', 'ch_cpi', 'ch_u', 'gbp_uah', 'chf_uah']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('карточка Швейцарии: только те показатели, что есть у страны', () => {
  const ch = { cc: 'ch', name: 'Швейцария', metrics: { policy: 'ch_policy', cpi: 'ch_cpi', u: 'ch_u', fx: 'chf_uah' } }
  assert.deepEqual(countryRows(ch, {}).map((r) => r.key), ['policy', 'cpi', 'u', 'fx'])
})

test('у рядов Японии, Китая, доходности Швейцарии и курсов иены и юаня есть подпись и цвет', () => {
  for (const id of ['ch_y10', 'jp_policy', 'jp_cpi', 'jp_u', 'jp_y10', 'jp_nikkei', 'cn_policy', 'cn_cpi', 'jpy_uah', 'cny_uah']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('карточка Китая: ставка, инфляция и курс; безработицы и доходности нет', () => {
  const cn = { cc: 'cn', name: 'Китай', metrics: { policy: 'cn_policy', cpi: 'cn_cpi', fx: 'cny_uah' } }
  assert.deepEqual(countryRows(cn, { cn_cpi: [{ d: '2026-09-01', v: 0.3 }] }).map((r) => r.text), ['—', '0,3 %', '—'])
})

test('у рядов России и Египта и курсов рубля и египетского фунта есть подпись и цвет', () => {
  for (const id of ['ru_policy', 'ru_cpi', 'eg_cpi', 'rub_uah', 'egp_uah']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('у рядов Польши и Турции и курсов злотого и лиры есть подпись и цвет', () => {
  for (const id of ['pl_policy', 'pl_cpi', 'pl_u', 'pl_y10', 'pln_uah', 'tr_policy', 'tr_cpi', 'tr_u', 'try_uah']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('карточка Турции без доходности 10 лет: строки только по имеющимся показателям', () => {
  const tr = { cc: 'tr', name: 'Турция', metrics: { policy: 'tr_policy', cpi: 'tr_cpi', u: 'tr_u', fx: 'try_uah' } }
  assert.deepEqual(countryRows(tr, {}).map((r) => r.key), ['policy', 'cpi', 'u', 'fx'])
  assert.equal(countryRows(tr, { tr_cpi: [{ d: '2026-10-01', v: 31.2 }] })[1].text, '31,2 %')
})

test('у рядов Норвегии, Швеции и Канады и курсов их валют есть подпись и цвет', () => {
  for (const cc of ['no', 'se', 'ca']) for (const k of ['policy', 'cpi', 'u', 'y10']) assert.ok(SERIES[`${cc}_${k}`]?.label && SERIES[`${cc}_${k}`]?.color, `${cc}_${k}`)
  for (const id of ['nok_uah', 'sek_uah', 'cad_uah', 'aud_uah', 'sar_uah', 'aed_uah', 'inr_uah']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('карточка Украины: резервы в млрд долларов и рост M3 в процентах', () => {
  const ua = { cc: 'ua', name: 'Украина', metrics: { policy: 'policy_rate', reserves: 'ua_reserves', m3: 'ua_m3_yoy' } }
  const rows = countryRows(ua, { ua_reserves: [{ d: '2026-09-01', v: 48.67 }], ua_m3_yoy: [{ d: '2026-09-01', v: 9.94 }] })
  assert.deepEqual(rows.map((r) => r.key), ['policy', 'reserves', 'm3'])
  assert.equal(rows[1].text, '48,67 млрд $')
  assert.equal(rows[2].text, '9,94 %')
  for (const id of ['ua_reserves', 'ua_m0', 'ua_m3', 'ua_m3_yoy']) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
})

test('Украина: графики под карточкой и их ряды попадают в запрос кеша', () => {
  const ua = { cc: 'ua', name: 'Украина', metrics: { policy: 'policy_rate', ppi: 'ua_ppi_yoy', ca: 'ua_ca' } }
  assert.ok(COUNTRY_PANELS.ua.length >= 2)
  const ids = countryIds([ua, { cc: 'us', metrics: { policy: 'fed_funds' } }])
  for (const id of ['policy_rate', 'ua_cpi_01', 'ua_cpi_11', 'ua_gs', 'ua_m3_yoy', 'fed_funds']) assert.ok(ids.includes(id), id)
  assert.equal(new Set(ids).size, ids.length)
  for (const p of COUNTRY_PANELS.ua) for (const id of p.ids) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
  for (let n = 1; n <= 12; n++) { const id = `ua_cpi_${String(n).padStart(2, '0')}`; assert.ok(SERIES[id]?.label, id) }
  const rows = countryRows(ua, { ua_ppi_yoy: [{ d: '2026-09-01', v: 5.7 }], ua_ca: [{ d: '2026-01-01', v: -1.02 }] })
  assert.deepEqual(rows.map((r) => r.text), ['—', '5,7 %', '-1,02 млрд $'])
})

test('высокая инфляция в карточке: без сокращений и потери знаков', () => {
  const eg = { cc: 'eg', name: 'Египет', metrics: { cpi: 'eg_cpi' } }
  assert.equal(countryRows(eg, { eg_cpi: [{ d: '2026-09-01', v: 165.2 }] })[0].text, '165,2 %')
})

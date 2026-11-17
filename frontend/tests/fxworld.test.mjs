import test from 'node:test'
import assert from 'node:assert/strict'
import { crossRate, rateText, crossMatrix, dxyIndex, fxWorldRows, DXY_CONST } from '../src/utils/fxworld.js'
import { SERIES } from '../src/utils/macro.js'
import { COUNTRY_METRICS } from '../src/utils/countries.js'

const D = '2026-11-10'
const Y = '2025-10-01'
const one = (v, d = D) => [{ d, v }]
// Паритет: все валюты по 1 доллару → индекс равен константе формулы
const parity = { usd_uah: one(40), eur_uah: one(40), gbp_uah: one(40), chf_uah: one(40), jpy_uah: one(40), cad_uah: one(40), sek_uah: one(40) }

test('кросс-курс через гривну и нет данных — null', () => {
  const s = { usd_uah: one(40), eur_uah: one(46) }
  assert.equal(crossRate(s, 'eur_uah', 'usd_uah', D), 46 / 40)
  assert.equal(crossRate(s, 'eur_uah', 'chf_uah', D), null)
  assert.equal(crossRate(null, 'eur_uah', 'usd_uah', D), null)
})

test('индекс доллара: паритет даёт константу, дорогой евро — индекс ниже', () => {
  assert.equal(dxyIndex(parity)[0].v, Math.round(DXY_CONST * 100) / 100)
  assert.ok(dxyIndex({ ...parity, eur_uah: one(48) })[0].v < DXY_CONST)
  assert.ok(dxyIndex({ ...parity, jpy_uah: one(20) })[0].v > DXY_CONST)
})

test('индекс доллара: нет любого ряда — пусто', () => {
  const { sek_uah, ...rest } = parity
  assert.deepEqual(dxyIndex(rest), [])
})

test('текст курса: знаков больше у мелких, запятая, нет данных — прочерк', () => {
  assert.equal(rateText(150), '150,00')
  assert.equal(rateText(3.65), '3,6500')
  assert.equal(rateText(0.0067), '0,006700')
  assert.equal(rateText(null), '—')
})

test('матрица: диагональ пустая, 1 EUR = 1,15 USD', () => {
  const m = crossMatrix({ usd_uah: one(40), eur_uah: one(46) }, D)
  assert.equal(m[0].cells[0], null)
  assert.equal(m[1].cells[0], 1.15)
  assert.equal(m[0].cells[1], 40 / 46)
})

test('строки таблицы: курс к доллару, изменение за год, разница ставок, реальный курс', () => {
  const countries = [
    { cc: 'us', name: 'США', currency: 'USD', metrics: { fx: 'usd_uah', policy: 'fed' } },
    { cc: 'pl', name: 'Польша', currency: 'PLN', metrics: { fx: 'pln_uah', policy: 'pl_policy', reer: 'reer_pl' } },
    { cc: 'xx', name: 'Без курса', currency: 'XXX', metrics: {} }
  ]
  const series = {
    usd_uah: [{ d: Y, v: 40 }, { d: D, v: 40 }],
    pln_uah: [{ d: Y, v: 10 }, { d: D, v: 11 }],
    fed: one(4), pl_policy: one(5.5),
    reer_pl: [{ d: '2025-09-01', v: 100 }, { d: '2026-10-01', v: 110 }]
  }
  const rows = fxWorldRows(countries, series, D)
  assert.deepEqual(rows.map((r) => r.cc), ['us', 'pl'])
  const pl = rows[1]
  assert.equal(pl.usdPer, 11 / 40)
  assert.ok(Math.abs(pl.chg1y - 10) < 1e-9)
  assert.equal(pl.carry, 1.5)
  assert.equal(pl.reer, 110)
  assert.ok(Math.abs(pl.reerChg1y - 10) < 1e-9)
  assert.equal(rows[0].usdPer, 1)
  assert.equal(rows[0].carry, 0)
  assert.equal(rows[0].reer, null)
})

test('строки таблицы: нет данных — null, пустой реестр — пусто', () => {
  assert.deepEqual(fxWorldRows([], {}, D), [])
  const rows = fxWorldRows([{ cc: 'us', name: 'США', currency: 'USD', metrics: { fx: 'usd_uah' } }, { cc: 'pl', name: 'П', currency: 'PLN', metrics: { fx: 'pln_uah' } }], {}, D)
  assert.equal(rows[1].chg1y, null)
  assert.equal(rows[1].carry, null)
})

test('реальные курсы: подпись, цвет и показатель карточки страны', () => {
  for (const cc of ['us', 'ez', 'gb', 'ch', 'jp', 'cn', 'ru', 'pl', 'tr', 'no', 'se', 'ca']) assert.ok(SERIES[`reer_${cc}`]?.label && SERIES[`reer_${cc}`]?.color, cc)
  assert.ok(SERIES.fx_dxy?.label)
  assert.ok(COUNTRY_METRICS.some((m) => m.key === 'reer'))
})

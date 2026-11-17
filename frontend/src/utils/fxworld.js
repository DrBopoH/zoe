import { valueAt, lastPoint, isoAt } from './macro.js'
import { fixedText } from './numfmt.js'

// Мировые валюты из курсов НБУ (гривен за единицу, ряды <ccy>_uah) и реестра стран с сервера. Кросс-курс = через гривну: A/B = (A к гривне) / (B к гривне).
// Чистые функции, без интерфейса. «Растёт» везде значит: валюта дорожает к доллару.

// Валюты матрицы кросс-курсов: код → ряд курса к гривне
export const MATRIX_CCY = [
  { code: 'USD', id: 'usd_uah' }, { code: 'EUR', id: 'eur_uah' }, { code: 'GBP', id: 'gbp_uah' }, { code: 'CHF', id: 'chf_uah' },
  { code: 'JPY', id: 'jpy_uah' }, { code: 'CNY', id: 'cny_uah' }, { code: 'PLN', id: 'pln_uah' }
]

// Сколько единиц B стоит одна единица A на дату (через курсы к гривне); нет данных — null
export function crossRate(series, idA, idB, iso) {
  const a = valueAt(series?.[idA], iso)
  const b = valueAt(series?.[idB], iso)
  return a > 0 && b > 0 ? a / b : null
}

// Курс для таблицы: 4 значащих цифры у мелких, две десятичные у крупных (иена к доллару — 150,00, злотый к доллару — 3,6500)
export function rateText(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const digits = v >= 100 ? 2 : v >= 1 ? 4 : v >= 0.01 ? 5 : 6
  return fixedText(v, digits)
}

// Матрица: строка A, столбец B — «1 A = x B». Диагональ — null.
export function crossMatrix(series, iso, ccy = MATRIX_CCY) {
  return ccy.map((a) => ({ code: a.code, cells: ccy.map((b) => (a.code === b.code ? null : crossRate(series, a.id, b.id, iso))) }))
}

// Индекс доллара по формуле ICE (DXY), посчитанный по курсам НБУ: константа 50,14348112 и степени-веса евро, иены, фунта, канадского доллара, кроны и франка.
// Каждая пара — через гривну; точки по датам курса доллара, остальные курсы — последние не позже даты.
export const DXY_CONST = 50.14348112
const DXY = [['eur_uah', 'usd_uah', -0.576], ['usd_uah', 'jpy_uah', 0.136], ['gbp_uah', 'usd_uah', -0.119], ['usd_uah', 'cad_uah', 0.091], ['usd_uah', 'sek_uah', 0.042], ['usd_uah', 'chf_uah', 0.036]]
export function dxyIndex(series) {
  const ids = new Set(DXY.flat().filter((x) => typeof x === 'string'))
  for (const id of ids) if (!series?.[id]?.length) return []
  const out = []
  for (const p of series.usd_uah) {
    let v = DXY_CONST
    let ok = true
    for (const [a, b, w] of DXY) {
      const r = crossRate(series, a, b, p.d)
      if (r === null) { ok = false; break }
      v *= Math.pow(r, w)
    }
    if (ok) out.push({ d: p.d, v: Math.round(v * 100) / 100 })
  }
  return out
}

// Строки таблицы «Мировые валюты»: по странам реестра, у которых есть курс к гривне. usdPer — долларов за единицу валюты (у доллара 1),
// chg1y — изменение этого курса за год, % (плюс — валюта окрепла к доллару), policy и carry — ставка центробанка и её разница со ставкой США, reer — реальный эффективный курс.
export function fxWorldRows(countries, series, today) {
  const us = (countries || []).find((c) => c.cc === 'us')
  const usdId = us?.metrics?.fx
  const fedNow = us?.metrics?.policy ? valueAt(series?.[us.metrics.policy], today) : null
  const yearAgo = isoAt(-1, today)
  const perUsd = (id, iso) => {
    const r = crossRate(series, id, usdId, iso)
    return r
  }
  return (countries || [])
    .filter((c) => c.metrics?.fx && usdId)
    .map((c) => {
      const now = perUsd(c.metrics.fx, today)
      const ago = perUsd(c.metrics.fx, yearAgo)
      const policy = c.metrics.policy ? valueAt(series?.[c.metrics.policy], today) : null
      const reerId = c.metrics.reer
      const reerNow = reerId ? lastPoint(series?.[reerId]) : null
      const reerAgo = reerId && reerNow ? valueAt(series[reerId], isoAt(-1, reerNow.d)) : null
      return {
        cc: c.cc, name: c.name, currency: c.currency,
        uah: valueAt(series?.[c.metrics.fx], today),
        usdPer: now,
        chg1y: now !== null && ago !== null ? (now / ago - 1) * 100 : null,
        policy,
        carry: policy !== null && fedNow !== null ? policy - fedNow : null,
        reer: reerNow ? reerNow.v : null,
        reerChg1y: reerNow && reerAgo ? (reerNow.v / reerAgo - 1) * 100 : null
      }
    })
}

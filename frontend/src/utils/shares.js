import { billionsText } from './macro.js'
import { numText } from './numfmt.js'

// Раздел «Акции компаний»: чистые функции над ответом /api/market/stock — ряды для графиков, мультипликаторы и сверка капитализации.
const BLN = 1e9
const DAY = 86400000
const ms = (iso) => Date.parse(`${String(iso).slice(0, 10)}T00:00:00Z`)

// Ряды для MacroPanel: цена в $, всё остальное в млрд $ (график подписывает «млрд $»)
export function stockSeries(d) {
  const f = d?.fundamentals || {}
  const bln = (a) => (a || []).map((p) => ({ d: p.d, v: p.v / BLN }))
  return {
    sh_price: (d?.prices || []).filter((p) => p.c > 0).map((p) => ({ d: p.d, v: p.c })),
    sh_cap: bln(d?.cap),
    sh_revenue: bln(f.revenue),
    sh_income: bln(f.net_income),
    sh_equity: bln(f.equity),
    sh_assets: bln(f.assets)
  }
}

const REPORT_MAX_AGE = 540 * DAY // отчёт старше полутора лет к сегодняшней капитализации не применяется

/** Мультипликаторы по сегодняшней капитализации и последним отчётам: P/S, P/E (только при прибыли), P/B (только при положительном капитале). Нет данных — null. */
export function stockRatios(d, todayIso) {
  const cap = d?.cap?.length ? d.cap[d.cap.length - 1] : null
  if (!cap || !(cap.v > 0)) return null
  const f = d.fundamentals || {}
  const last = (a) => {
    const p = a?.length ? a[a.length - 1] : null
    return p && ms(todayIso) - ms(p.d) <= REPORT_MAX_AGE ? p : null
  }
  const rev = last(f.revenue), inc = last(f.net_income), eq = last(f.equity)
  return {
    cap: cap.v,
    capDate: cap.d,
    revenue: rev, income: inc, equity: eq,
    ps: rev && rev.v > 0 ? cap.v / rev.v : null,
    pe: inc && inc.v > 0 ? cap.v / inc.v : null,
    pb: eq && eq.v > 0 ? cap.v / eq.v : null
  }
}

/**
 * Сверка расчётной капитализации (цена × акции из отчёта SEC) с текущей по Nasdaq. У компаний с несколькими классами акций (GOOGL, BRK) в отчёте может быть
 * только часть акций, и расчёт уйдёт вниз. Расхождение больше 25 % — предупреждение; последняя точка старше трёх месяцев — сверять не с чем (null).
 */
export function capCheck(marketCap, cap, todayIso) {
  const last = cap?.length ? cap[cap.length - 1] : null
  if (!last || !(marketCap > 0) || ms(todayIso) - ms(last.d) > 93 * DAY) return null
  const diff = last.v / marketCap - 1 // обе капитализации в долларах; в ответе — в млрд $ для подписи
  return { diff: diff * 100, ok: Math.abs(diff) <= 0.25, nasdaq: marketCap / BLN, own: last.v / BLN }
}

/** Доллары словами: 4,1e12 → «4,1 трлн $», убыток −5e9 → «−5 млрд $»; нет значения — «—». */
export const usdText = (usd) => (usd === null || usd === undefined || !isFinite(usd) || usd === 0 ? '—' : `${usd < 0 ? '−' : ''}${billionsText(Math.abs(usd) / BLN)}`)
export const ratioText = (v) => (v === null || v === undefined || !isFinite(v) ? '—' : numText(v, 1))

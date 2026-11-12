import { annualChange, lastPoint, realRate } from './macro.js'

// Доллар как общая мера: курсы к доллару, доходность бумаг в долларах, капитал в долларовом эквиваленте. Чистые функции.
// Курсы НБУ: usd_uah и eur_uah — гривен за единицу валюты (ряды из /api/macro, FX_IDS).

/** Текущие курсы из макро-рядов: { usdUah, eurUah } (null — ряда нет). Евро в долларах = eurUah / usdUah. */
export function fxRates(series) {
  const usdUah = lastPoint(series && series.usd_uah)?.v ?? null
  const eurUah = lastPoint(series && series.eur_uah)?.v ?? null
  return { usdUah: usdUah > 0 ? usdUah : null, eurUah: eurUah > 0 ? eurUah : null }
}

/** Сумма в долларах: 840 как есть (без превращений), 980 по курсу НБУ, 978 через гривну; остальное и отсутствие курса — null. */
export function toUsd(code, amount, rates) {
  const v = Number(amount)
  if (!isFinite(v)) return null
  if (code === 840) return v
  if (code === 980) return rates && rates.usdUah ? v / rates.usdUah : null
  if (code === 978) return rates && rates.usdUah && rates.eurUah ? (v * rates.eurUah) / rates.usdUah : null
  return null
}

/**
 * Доходность облигации в долларах и реальная (к инфляции США), % годовых — чтобы сравнивать бумаги любых валют на одной шкале.
 * rate — номинальная доходность бумаги, currency — её валюта (UAH | USD | EUR).
 * Гривневая: доллар дорожает на devUsd % в год (среднегодовое за прошлые годы) → (1 + r) / (1 + devUsd) − 1.
 * Евро: евро к доллару меняется на eurGrowth % в год → (1 + r)(1 + g) − 1. Доллар — как есть.
 * Возвращает { usd, realUsd } (null, если для расчёта нет данных).
 */
export function bondDollarYield({ rate, currency, devUsd = null, eurGrowth = null, usInflation = null }) {
  if (!(typeof rate === 'number' && isFinite(rate))) return { usd: null, realUsd: null }
  let usd = null
  if (currency === 'USD') usd = rate
  else if (currency === 'UAH') usd = realRate(rate, devUsd)
  else if (currency === 'EUR' && typeof eurGrowth === 'number' && isFinite(eurGrowth)) usd = ((1 + rate / 100) * (1 + eurGrowth / 100) - 1) * 100
  return { usd, realUsd: usd === null ? null : realRate(usd, usInflation) }
}

/** Допущения для доходности в долларах: средний рост доллара к гривне и евро к доллару за years лет. */
export function dollarAssumptions(series, todayIso, years = 5) {
  const usd = annualChange(series && series.usd_uah, todayIso, years)
  const eurUah = annualChange(series && series.eur_uah, todayIso, years)
  const eurGrowth = usd === null || eurUah === null ? null : ((1 + eurUah / 100) / (1 + usd / 100) - 1) * 100
  return { devUsd: usd, eurGrowth, usInflation: lastPoint(series && series.us_cpi_yoy)?.v ?? null, years }
}

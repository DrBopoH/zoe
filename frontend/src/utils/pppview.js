import { lastPoint, valueAt } from './macro.js'
import { purchasingPower, realRateIndex, priceLevel, pppDeviation, incomeRatio, salaryInIntl, bigMacRows, burgersPerMonth } from './ppp.js'

// Модель страницы «Покупательная способность»: из рядов источника «ppp» и выбора человека (страна, окно, сумма) собирает всё, что рисует экран. Чистые функции.
// Правила стран: у еврозоны нет индекса цен и PPP-коэффициента — берутся немецкие (de); у США индекс цен помесячный (ppp_us_cpi).
export const PPP_COUNTRIES = [
  { cc: 'ua', name: 'Украина', cur: 'UAH', sym: '₴' },
  { cc: 'us', name: 'США', cur: 'USD', sym: '$' },
  { cc: 'ez', name: 'Еврозона', cur: 'EUR', sym: '€' },
  { cc: 'de', name: 'Германия', cur: 'EUR', sym: '€' },
  { cc: 'gb', name: 'Великобритания', cur: 'GBP', sym: '£' },
  { cc: 'ch', name: 'Швейцария', cur: 'CHF', sym: 'CHF' },
  { cc: 'jp', name: 'Япония', cur: 'JPY', sym: '¥' },
  { cc: 'cn', name: 'Китай', cur: 'CNY', sym: '¥' },
  { cc: 'ru', name: 'Россия', cur: 'RUB', sym: '₽' },
  { cc: 'pl', name: 'Польша', cur: 'PLN', sym: 'zł' },
  { cc: 'tr', name: 'Турция', cur: 'TRY', sym: '₺' },
  { cc: 'no', name: 'Норвегия', cur: 'NOK', sym: 'kr' },
  { cc: 'se', name: 'Швеция', cur: 'SEK', sym: 'kr' },
  { cc: 'ca', name: 'Канада', cur: 'CAD', sym: 'C$' },
  { cc: 'eg', name: 'Египет', cur: 'EGP', sym: 'E£' }
]
export const countryOf = (cc) => PPP_COUNTRIES.find((c) => c.cc === cc) || null
const base = (cc) => (cc === 'ez' ? 'de' : cc) // индекс цен и PPP-коэффициент еврозоны — немецкие

export const cpiPoints = (series, cc) => (cc === 'us' ? series?.ppp_us_cpi : series?.[`ppp_cpi_${base(cc)}`]) || []
export const convPoints = (series, cc) => series?.[`ppp_conv_${base(cc)}`] || []
export const fxPoints = (series, cc) => series?.[`ppp_fx_${cc}`] || []

// Окно: сколько лет назад (от года today), точка отсчёта — 1 июля того года (на неё приходится годовая точка Всемирного банка)
export const startIso = (today, years) => `${Number(today.slice(0, 4)) - years}-07-01`

// Темп инфляции для продолжения индекса после последней годовой точки: у Украины — текущая годовая инфляция НБУ (cpi_yoy), у остальных — по их же ряду
export function growthOf(cc, macro) {
  if (cc !== 'ua') return null
  const v = lastPoint(macro?.cpi_yoy)?.v
  return typeof v === 'number' ? v / 100 : null
}

// Ряд покупательной способности единицы валюты: 100 на старте, дальше сколько осталось. Точки — раз в полгода и сегодня.
function powerSeries(points, from, today, growth) {
  const out = []
  const y0 = Number(from.slice(0, 4)), y1 = Number(today.slice(0, 4))
  const dates = []
  for (let y = y0; y <= y1; y++) for (const m of ['01', '07']) {
    const d = `${y}-${m}-01`
    if (d >= from && d <= today) dates.push(d)
  }
  if (dates[dates.length - 1] !== today) dates.push(today)
  for (const d of dates) {
    const pp = purchasingPower(points, from, d, growth)
    if (pp) out.push({ d, v: pp.ratio * 100 })
  }
  return out
}

/**
 * Что происходит с деньгами страны за окно: сумма тогда и сегодня в ценах другой даты, накопленная и среднегодовая инфляция,
 * ряд покупательной способности и реальный курс к доллару. series — ряды источника ppp, macro — ряды НБУ (для темпа Украины).
 */
export function pppModel({ series, macro, cc, years, today, amount = 1000 }) {
  const from = startIso(today, years)
  const points = cpiPoints(series, cc)
  const growth = growthOf(cc, macro)
  const pp = purchasingPower(points, from, today, growth)
  const power = powerSeries(points, from, today, growth)
  const rer = cc === 'us' ? [] : realRateIndex(fxPoints(series, cc), points, cpiPoints(series, 'us'), from)
  return {
    from, pp,
    // сколько стоила бы сегодня корзина, которая тогда стоила amount, и сколько тогда стоило бы то, что amount покупает сегодня
    needNow: pp ? amount * (1 + pp.inflation / 100) : null,
    worthThen: pp ? amount * pp.ratio : null,
    power, rer
  }
}

/** Таблица стран: уровень цен относительно США, отклонение от PPP, доход на душу (PPP и номинальный). Последний год данных, отсортировано по уровню цен (дорогие сверху). */
export function levelRows(series, today) {
  return PPP_COUNTRIES.filter((c) => c.cc !== 'us').concat(countryOf('us')).map((c) => {
    const iso = today
    const lvl = c.cc === 'us' ? 1 : priceLevel(convPoints(series, c.cc), fxPoints(series, c.cc), iso)
    return {
      cc: c.cc, name: c.name,
      level: lvl,
      dev: c.cc === 'us' ? 0 : pppDeviation(convPoints(series, c.cc), fxPoints(series, c.cc), iso),
      gdpPpp: valueAt(series?.[`ppp_gdp_${c.cc}`], iso),
      gdpNom: valueAt(series?.[`ppp_gdpn_${c.cc}`], iso)
    }
  }).filter((r) => r.level !== null || r.gdpPpp !== null).sort((a, b) => (b.level ?? -1) - (a.level ?? -1))
}

/** Big Mac по странам и «бургеров в месяц» на среднедушевой доход (номинальный ВВП на душу ÷ 12 на цену бургера). Сортировка — по цене. */
export function burgerRows(series, today) {
  const ccs = PPP_COUNTRIES.map((c) => c.cc).filter((cc) => cc !== 'de')
  return bigMacRows(series, ccs).filter((r) => r.price !== null).map((r) => {
    const gdpn = valueAt(series?.[`ppp_gdpn_${r.cc}`], today)
    return { ...r, name: countryOf(r.cc)?.name || r.cc, perMonth: gdpn ? burgersPerMonth(gdpn / 12, r.price) : null }
  }).sort((a, b) => b.price - a.price)
}

/**
 * Две страны: оклад salaryA в валюте A → сколько в валюте B даёт ту же покупательную способность по PPP, и сколько это по рыночному курсу.
 * Больше всего показывает разрыв: в бедной стране то же самое жильё и еда стоят дешевле, чем следует из курса.
 */
export function compareCountries({ series, a, b, salaryA, today }) {
  const intl = salaryInIntl(salaryA, convPoints(series, a), today)
  const convB = valueAt(convPoints(series, b), today)
  const fxA = a === 'us' ? 1 : valueAt(fxPoints(series, a), today)
  const fxB = b === 'us' ? 1 : valueAt(fxPoints(series, b), today)
  const income = incomeRatio(series?.[`ppp_gdp_${a}`], series?.[`ppp_gdp_${b}`], series?.[`ppp_gdpn_${a}`], series?.[`ppp_gdpn_${b}`], today)
  return {
    intl,
    inB: intl !== null && convB > 0 ? intl * convB : null, // оклад в валюте B с той же покупательной способностью
    usdA: salaryA > 0 && fxA > 0 ? salaryA / fxA : null, // оклад A по рыночному курсу, $
    inBMarket: salaryA > 0 && fxA > 0 && fxB > 0 ? (salaryA / fxA) * fxB : null, // то же в валюте B по рыночному курсу
    income
  }
}

/** Реальная доходность: номинальная nominalPct годовых при инфляции inflationPct годовых; итог за years лет в деньгах будущего и в сегодняшних. */
export function realReturn({ nominalPct, inflationPct, years, amount }) {
  if (![nominalPct, inflationPct, years, amount].every((v) => typeof v === 'number' && isFinite(v)) || years < 0) return null
  const real = ((1 + nominalPct / 100) / (1 + inflationPct / 100) - 1) * 100
  const grown = amount * Math.pow(1 + nominalPct / 100, years)
  const today = amount * Math.pow(1 + real / 100, years)
  return { real, grown, today, beats: real > 0 }
}

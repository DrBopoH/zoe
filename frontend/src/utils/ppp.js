import { lastPoint, valueAt } from './macro.js'

// Покупательная способность: формулы поверх рядов источника «ppp» (Всемирный банк, FRED, Big Mac). Чистые функции, без интерфейса.
// Ряды цен у Всемирного банка годовые (точка на 1 июля = годовое среднее), у США — помесячные: поэтому индекс цены на любую дату считается
// интерполяцией (геометрической — равный темп роста между точками), а после последней точки продолжается с темпом инфляции (последний известный годовой, если задан).
const DAY = 86400000
const YEAR = 365.25 * DAY
const ms = (iso) => Date.parse(`${iso}T00:00:00Z`)

/** Индекс цен на дату: между точками — геометрическая интерполяция, после последней — продолжение по темпу growth (доля за год; нет — темп последних двух точек), раньше первой — null. */
export function priceIndex(points, iso, growth = null) {
  const n = points?.length || 0
  if (!n || !iso || iso < points[0].d) return null
  const last = points[n - 1]
  if (iso >= last.d) {
    if (iso === last.d) return last.v
    let g = growth
    if (g === null || g === undefined || !isFinite(g)) {
      if (n < 2) return last.v
      const prev = points[n - 2]
      g = Math.pow(last.v / prev.v, YEAR / (ms(last.d) - ms(prev.d))) - 1
    }
    return last.v * Math.pow(1 + g, (ms(iso) - ms(last.d)) / YEAR)
  }
  let lo = 0, hi = n - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (points[mid].d <= iso) lo = mid
    else hi = mid - 1
  }
  const a = points[lo], b = points[lo + 1]
  if (a.d === iso) return a.v
  const t = (ms(iso) - ms(a.d)) / (ms(b.d) - ms(a.d))
  return a.v * Math.pow(b.v / a.v, t)
}

/**
 * Что случилось с деньгами страны между from и to: ratio — сколько товаров на единицу валюты осталось (1 = столько же, 0,5 — вдвое меньше),
 * inflation — накопленная инфляция, %, perYear — среднегодовая, %. Нет данных на одну из дат — null.
 */
export function purchasingPower(points, from, to, growth = null) {
  const a = priceIndex(points, from, growth), b = priceIndex(points, to, growth)
  if (!(a > 0) || !(b > 0)) return null
  const years = (ms(to) - ms(from)) / YEAR
  return { ratio: a / b, inflation: (b / a - 1) * 100, perYear: years > 0 ? (Math.pow(b / a, 1 / years) - 1) * 100 : null }
}

/** Сколько надо денег на дату to, чтобы купить столько же, сколько amount на дату from (то есть amount «в ценах to»). */
export function realValue(amount, points, from, to, growth = null) {
  const pp = purchasingPower(points, from, to, growth)
  return pp ? amount * (1 + pp.inflation / 100) : null
}

/**
 * Реальный курс к доллару: индекс по годам от базовой даты (=100). q = E · P_США / P_страны, где E — местной валюты за доллар.
 * Выше 100 — местная валюта в реальном выражении дешевле, чем в базе; ниже — дороже (подорожала сильнее инфляционной разницы).
 * fx — курс по годам, cpi — индекс цен страны, usCpi — индекс цен США; считается по датам курса.
 */
export function realRateIndex(fx, cpi, usCpi, baseIso) {
  const e0 = valueAt(fx, baseIso), p0 = valueAt(cpi, baseIso), u0 = valueAt(usCpi, baseIso)
  if (!(e0 > 0) || !(p0 > 0) || !(u0 > 0)) return []
  const out = []
  for (const pt of fx || []) {
    if (pt.d < baseIso) continue
    const p = valueAt(cpi, pt.d), u = valueAt(usCpi, pt.d)
    if (!(p > 0) || !(u > 0)) continue
    out.push({ d: pt.d, v: ((pt.v / e0) * (u / u0)) / (p / p0) * 100 })
  }
  return out
}

/** Уровень цен страны относительно США (1 — как в США): PPP-коэффициент к рыночному курсу. Украина ≈ 0,28 — цены вчетверо ниже американских. */
export function priceLevel(conv, fx, iso) {
  const c = valueAt(conv, iso), e = valueAt(fx, iso)
  return c > 0 && e > 0 ? c / e : null
}

/** Отклонение валюты от PPP, %: минус — рыночный курс «слабее» паритета (цены в долларах ниже американских), плюс — сильнее. */
export function pppDeviation(conv, fx, iso) {
  const lvl = priceLevel(conv, fx, iso)
  return lvl === null ? null : (lvl - 1) * 100
}

/** Сравнение доходов двух стран: по PPP (что можно купить) и номинальное (в долларах по курсу). Значение больше 1 — у A больше, чем у B. */
export function incomeRatio(gdpPppA, gdpPppB, gdpNomA, gdpNomB, iso) {
  const pa = valueAt(gdpPppA, iso), pb = valueAt(gdpPppB, iso), na = valueAt(gdpNomA, iso), nb = valueAt(gdpNomB, iso)
  return { ppp: pa > 0 && pb > 0 ? pa / pb : null, nominal: na > 0 && nb > 0 ? na / nb : null }
}

/** Зарплата в «международных долларах» (на что она хватает по местным ценам): оклад в местной валюте на PPP-коэффициент. */
export function salaryInIntl(salaryLocal, conv, iso) {
  const c = valueAt(conv, iso)
  return c > 0 ? salaryLocal / c : null
}

/** Big Mac по странам: последняя цена в долларах и отклонение от американской, %. Порядок — как в cc. */
export function bigMacRows(series, ccs) {
  return ccs.map((cc) => {
    const price = lastPoint(series?.[`bm_usd_${cc}`])
    const raw = lastPoint(series?.[`bm_raw_${cc}`])
    return { cc, price: price ? price.v : null, raw: raw ? raw.v : null, date: price?.d || raw?.d || null }
  })
}

/** Сколько Биг Маков в месяц хватает на доход: доход в долларах на цену в долларах. */
export const burgersPerMonth = (incomeUsd, priceUsd) => (incomeUsd > 0 && priceUsd > 0 ? incomeUsd / priceUsd : null)

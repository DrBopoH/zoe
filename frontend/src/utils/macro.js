import { scaleTicks, timeTicks } from './growthChart.js'
import { robustRanges, scaleOf } from './chartView.js'

// Макро НБУ: ключевые ставки и инфляция (ряды из /api/macro). Чистые функции — без интерфейса.
// Ставки приходят датами смены значения (значение действует до следующей точки), инфляция — помесячно.

export const SERIES = {
  policy_rate: { label: 'Облікова ставка', color: '#34d399', step: true },
  cpi_yoy: { label: 'Инфляция г/г', color: '#fbbf24', step: false },
  core_yoy: { label: 'Базовая инфляция г/г', color: '#fb923c', step: false, thin: true },
  overnight_cd: { label: 'Сертификаты овернайт', color: '#38bdf8', step: true },
  overnight_loans: { label: 'Кредиты овернайт', color: '#a78bfa', step: true },
  cpi_mom: { label: 'Инфляция м/м', color: '#fbbf24', step: false },
  core_mom: { label: 'Базовая м/м', color: '#fb923c', step: false, thin: true },
  usd_uah: { label: 'Доллар США', color: '#34d399', step: false },
  eur_uah: { label: 'Евро', color: '#38bdf8', step: false },
  xau_uah: { label: 'Золото, ₴ за унцию', color: '#fbbf24', step: false },
  xau_usd: { label: 'Золото, $ за унцию', color: '#fbbf24', step: false },
  fed_funds: { label: 'Ставка ФРС (факт)', color: '#a78bfa', step: false },
  fed_target: { label: 'ФРС: верх коридора', color: '#34d399', step: true },
  ust_10y: { label: 'Treasury 10 л.', color: '#38bdf8', step: false, thin: true },
  eu_cpi_yoy: { label: 'Инфляция еврозоны г/г', color: '#38bdf8', step: false },
  us_cpi_yoy: { label: 'Инфляция США г/г', color: '#fbbf24', step: false },
  us_cpi_mom: { label: 'Инфляция США м/м', color: '#fb923c', step: false },
  us_unrate: { label: 'Безработица США', color: '#38bdf8', step: false },
  us_gdp: { label: 'Рост ВВП США (в год)', color: '#34d399', step: true },
  us_spread: { label: 'Кривая: 10 л. − 2 г.', color: '#a78bfa', step: false },
  // как у индекса страха и жадности: середина (около 20, обычный фон) полупрозрачный серый, высокий VIX = страх голубой, очень низкий = самоуспокоенность янтарная
  us_vix: { label: 'VIX (индекс страха)', color: '#94a3b8', step: false, gradient: { from: 10, to: 50, stops: [[0, '#fbbf24', 1], [0.25, '#94a3b8', 0.4], [0.75, '#38bdf8', 1], [1, '#38bdf8', 1]] } },
  // светофор: чем оптимистичнее люди, тем зеленее (от 50 красный, около 75 жёлтый, от 100 зелёный)
  us_sentiment: { label: 'Настроения потребителей', color: '#fbbf24', step: false, noLog: true, gradient: { from: 50, to: 100, stops: [[0, '#f87171', 1], [0.5, '#fbbf24', 1], [1, '#34d399', 1]] } },
  ust_1m: { label: 'Treasury 1 мес.', color: '#64748b', step: false, thin: true },
  ust_3m: { label: 'Treasury 3 мес.', color: '#fbbf24', step: false },
  ust_6m: { label: 'Treasury 6 мес.', color: '#fb923c', step: false, thin: true },
  ust_1y: { label: 'Treasury 1 г.', color: '#f472b6', step: false, thin: true },
  ust_2y: { label: 'Treasury 2 г.', color: '#a78bfa', step: false },
  ust_3y: { label: 'Treasury 3 г.', color: '#818cf8', step: false, thin: true },
  ust_5y: { label: 'Treasury 5 л.', color: '#2dd4bf', step: false, thin: true },
  ust_7y: { label: 'Treasury 7 л.', color: '#4ade80', step: false, thin: true },
  ust_20y: { label: 'Treasury 20 л.', color: '#f87171', step: false, thin: true },
  ust_30y: { label: 'Treasury 30 л.', color: '#34d399', step: false },
  us_spread3m: { label: 'Кривая: 10 л. − 3 мес.', color: '#a78bfa', step: false },
  us_real5: { label: 'Реальная доходность 5 л. (TIPS)', color: '#2dd4bf', step: false, thin: true },
  us_real10: { label: 'Реальная доходность 10 л. (TIPS)', color: '#38bdf8', step: false },
  us_be5: { label: 'Брейкивен 5 л.', color: '#fb923c', step: false, thin: true },
  us_be10: { label: 'Брейкивен 10 л.', color: '#fbbf24', step: false },
  us_fwd5y5y: { label: 'Инфляция 5y5y', color: '#a78bfa', step: false },
  us_termprem: { label: 'Срочная премия 10 л.', color: '#f472b6', step: false },
  us_mortgage: { label: 'Ипотека 30 л.', color: '#34d399', step: false },
  us_corecpi_yoy: { label: 'Базовый CPI г/г', color: '#fb923c', step: false },
  us_corecpi_mom: { label: 'Базовый CPI м/м', color: '#fb923c', step: false },
  us_pce_yoy: { label: 'PCE г/г', color: '#a78bfa', step: false, thin: true },
  us_corepce_yoy: { label: 'Базовый PCE г/г', color: '#34d399', step: false },
  us_ppi_yoy: { label: 'Цены производителей г/г', color: '#f472b6', step: false },
  us_shelter_yoy: { label: 'Жильё в CPI г/г', color: '#38bdf8', step: false },
  us_nfp: { label: 'Прирост занятых за месяц', color: '#34d399', step: false, noLog: true },
  us_claims: { label: 'Заявки на пособие (неделя)', color: '#fbbf24', step: false },
  us_jolts: { label: 'Открытые вакансии', color: '#38bdf8', step: false },
  us_wages_yoy: { label: 'Зарплаты г/г', color: '#34d399', step: false },
  us_u6: { label: 'Безработица U-6', color: '#fb923c', step: false },
  us_eci_yoy: { label: 'Стоимость рабочей силы г/г', color: '#a78bfa', step: false, thin: true },
  us_retail_yoy: { label: 'Розничные продажи г/г', color: '#38bdf8', step: false },
  us_indpro_yoy: { label: 'Промышленность г/г', color: '#fbbf24', step: false },
  us_housing: { label: 'Начато строительство жилья', color: '#a78bfa', step: false },
  us_homeprice_yoy: { label: 'Цены на дома г/г', color: '#34d399', step: false },
  us_m2: { label: 'Денежная масса M2', color: '#38bdf8', step: false },
  us_m2_yoy: { label: 'M2 г/г', color: '#38bdf8', step: false },
  us_fedbs: { label: 'Баланс ФРС', color: '#a78bfa', step: false },
  us_rrp: { label: 'Обратное РЕПО', color: '#fbbf24', step: false, thin: true },
  us_tga: { label: 'Счёт Казначейства', color: '#f472b6', step: false, thin: true },
  us_netliq: { label: 'Чистая ликвидность', color: '#34d399', step: false },
  ez_policy: { label: 'ЕЦБ: ставка по депозитам', color: '#a78bfa', step: true },
  ez_mro: { label: 'ЕЦБ: основная ставка', color: '#34d399', step: true },
  ez_estr: { label: '€STR (овернайт)', color: '#38bdf8', step: false, thin: true },
  ez_y2: { label: 'Еврозона 2 г. (AAA)', color: '#fbbf24', step: false, thin: true },
  ez_y10: { label: 'Еврозона 10 л. (AAA)', color: '#38bdf8', step: false },
  ez_cpi_core: { label: 'Базовая инфляция еврозоны г/г', color: '#fb923c', step: false, thin: true },
  ez_u: { label: 'Безработица еврозоны', color: '#38bdf8', step: false },
  ez_gdp: { label: 'Рост ВВП еврозоны (за квартал)', color: '#34d399', step: true },
  de_cpi: { label: 'Германия: инфляция г/г', color: '#fbbf24', step: false },
  de_u: { label: 'Германия: безработица', color: '#fbbf24', step: false },
  de_y10: { label: 'Германия: 10 лет', color: '#fbbf24', step: false },
  fr_cpi: { label: 'Франция: инфляция г/г', color: '#38bdf8', step: false },
  fr_u: { label: 'Франция: безработица', color: '#38bdf8', step: false },
  fr_y10: { label: 'Франция: 10 лет', color: '#38bdf8', step: false },
  it_cpi: { label: 'Италия: инфляция г/г', color: '#34d399', step: false },
  it_u: { label: 'Италия: безработица', color: '#34d399', step: false },
  it_y10: { label: 'Италия: 10 лет', color: '#34d399', step: false },
  es_cpi: { label: 'Испания: инфляция г/г', color: '#fb923c', step: false },
  es_u: { label: 'Испания: безработица', color: '#fb923c', step: false },
  es_y10: { label: 'Испания: 10 лет', color: '#fb923c', step: false },
  nl_cpi: { label: 'Нидерланды: инфляция г/г', color: '#a78bfa', step: false },
  nl_u: { label: 'Нидерланды: безработица', color: '#a78bfa', step: false },
  nl_y10: { label: 'Нидерланды: 10 лет', color: '#a78bfa', step: false },
  it_bund_spread: { label: 'Италия − Германия, 10 лет', color: '#34d399', step: false },
  fr_bund_spread: { label: 'Франция − Германия, 10 лет', color: '#38bdf8', step: false },
  es_bund_spread: { label: 'Испания − Германия, 10 лет', color: '#fb923c', step: false },
  gb_policy: { label: 'Банк Англии: ставка', color: '#a78bfa', step: true },
  gb_sonia: { label: 'SONIA (овернайт)', color: '#38bdf8', step: false, thin: true },
  gb_y10: { label: 'Gilt 10 лет', color: '#fbbf24', step: false },
  gb_cpi: { label: 'Инфляция Великобритании г/г', color: '#fb923c', step: false },
  gb_u: { label: 'Безработица Великобритании', color: '#38bdf8', step: false },
  gb_gdp: { label: 'Рост ВВП Великобритании (за квартал)', color: '#34d399', step: true },
  ch_policy: { label: 'SNB: ставка', color: '#a78bfa', step: true },
  ch_cpi: { label: 'Инфляция Швейцарии г/г', color: '#fb923c', step: false },
  ch_u: { label: 'Безработица Швейцарии', color: '#38bdf8', step: false },
  gbp_uah: { label: 'Фунт стерлингов', color: '#a78bfa', step: false },
  chf_uah: { label: 'Швейцарский франк', color: '#f87171', step: false },
  us_sofr: { label: 'SOFR', color: '#38bdf8', step: false },
  us_hy: { label: 'Спред HY (высокий доход)', color: '#f87171', step: false },
  us_ig: { label: 'Спред IG (инвест. рейтинг)', color: '#38bdf8', step: false },
  us_nfci: { label: 'Финансовые условия (NFCI)', color: '#a78bfa', step: false },
  us_usd: { label: 'Индекс доллара (широкий)', color: '#fbbf24', step: false },
  us_debt_gdp: { label: 'Госдолг к ВВП', color: '#fb923c', step: false },
  crypto_btc: { label: 'Bitcoin', color: '#fb923c', step: false },
  crypto_eth: { label: 'Ethereum', color: '#a78bfa', step: false },
  crypto_bnb: { label: 'BNB', color: '#fbbf24', step: false },
  crypto_sol: { label: 'Solana', color: '#34d399', step: false },
  // gradient: цвет линии зависит от значения (stops: [доля от from до to, цвет, прозрачность]); noLog — ограниченный индекс, логарифм не нужен
  crypto_fng: { label: 'Страх и жадность', color: '#94a3b8', step: false, noLog: true, gradient: { from: 0, to: 100, stops: [[0, '#38bdf8', 1], [0.5, '#94a3b8', 0.4], [1, '#fbbf24', 1]] } },
  stock_sp500: { label: 'S&P 500', color: '#38bdf8', step: false },
  stock_nasdaq: { label: 'NASDAQ Composite', color: '#a78bfa', step: false },
  stock_dow: { label: 'Dow Jones', color: '#fbbf24', step: false },
  stock_sp500_dd: { label: 'S&P 500: просадка от максимума', color: '#38bdf8', step: false },
  stock_nasdaq_dd: { label: 'NASDAQ: просадка от максимума', color: '#a78bfa', step: false },
  stock_dow_dd: { label: 'Dow Jones: просадка от максимума', color: '#fbbf24', step: false },
  crypto_btc_dd: { label: 'Bitcoin: просадка от максимума', color: '#fb923c', step: false },
  crypto_eth_dd: { label: 'Ethereum: просадка от максимума', color: '#a78bfa', step: false }
}

// Число для оси и подсказки по единице ряда: «%» — процент, «₴»/«$» — деньги (тысячи сокращаются: 183 562 → «184 тыс.»)
export function unitText(v, unit = '%') {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  if (unit === '%') return plainPct(v)
  const abs = Math.abs(v)
  const body = abs >= 10000 ? `${Math.round(v / 1000)} тыс.` : abs >= 100 ? String(Math.round(v)) : String(Math.round(v * 100) / 100).replace('.', ',')
  return `${body} ${unit}`
}
// Метка оси: то же, но короче (без десятых у дробных, чтобы подписи не слипались)
export const axisText = (v, unit = '%') => {
  if (unit === '%') return `${v} %`
  if (Math.abs(v) >= 10000) return `${Math.round(v / 1000)}к ${unit}`
  return unitText(v, unit)
}

// Золото в долларах: гривенная цена унции на курс доллара той же даты (курс — последний не позже даты)
export function goldUsd(series) {
  const g = series && series.xau_uah, u = series && series.usd_uah
  if (!g || !u || !g.length || !u.length) return []
  const out = []
  for (const p of g) {
    const r = valueAt(u, p.d)
    if (r) out.push({ d: p.d, v: p.v / r })
  }
  return out
}

// Среднегодовое изменение ряда за последние years лет, % (сложный процент). Нет данных так далеко назад — null.
export function annualChange(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  if (then === null || !(then > 0) || !(last.v > 0)) return null
  return (Math.pow(last.v / then, 1 / years) - 1) * 100
}

const DAY = 86400000
const ms = (iso) => Date.parse(`${iso}T00:00:00Z`)

// Значение ряда на дату: последняя точка не позже даты; раньше первой точки — null. Ряд отсортирован по возрастанию.
export function valueAt(points, iso) {
  if (!points || !points.length || !iso || iso < points[0].d) return null
  let lo = 0, hi = points.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (points[mid].d <= iso) lo = mid
    else hi = mid - 1
  }
  return points[lo].v
}

// Последняя точка ряда или null
export const lastPoint = (points) => (points && points.length ? points[points.length - 1] : null)

// «2026-11-13» → «13.11.2026»; пусто для пустой даты
export const dateOf = (iso) => (iso ? iso.split('-').reverse().join('.') : '')
// Есть ли хоть одна точка хоть в одном ряду (пока нет — на экране подсказка «Нажми Обновить»)
export const hasPoints = (series) => Object.values(series || {}).some((a) => a && a.length)

// Реальная доходность, % (по Фишеру): (1 + номинальная) / (1 + инфляция) − 1. Нет инфляции или ставки — null.
export function realRate(nominal, inflation) {
  if (!(typeof nominal === 'number' && isFinite(nominal)) || !(typeof inflation === 'number' && isFinite(inflation))) return null
  return ((1 + nominal / 100) / (1 + inflation / 100) - 1) * 100
}

/** Инфляция г/г (последний известный месяц) в валюте бумаги: UAH — НБУ, USD — США, EUR — еврозона (HICP). Нет данных или чужая валюта — null. */
export function inflationFor(currency, series) {
  const id = { UAH: 'cpi_yoy', USD: 'us_cpi_yoy', EUR: 'eu_cpi_yoy' }[currency]
  return id ? lastPoint(series && series[id])?.v ?? null : null
}

// Текущая инфляция для расчёта реальной доходности: последний известный месяц (г/г). null — данных нет.
export const currentInflation = (series) => lastPoint(series && series.cpi_yoy)?.v ?? null

// Знаковый процент с запятой: «+9,4 %», «−1,2 %», «0 %»
export function signedPct(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const r = Math.round(v * 10) / 10
  const s = String(Math.abs(r)).replace('.', ',')
  return `${r > 0 ? '+' : r < 0 ? '−' : ''}${s} %`
}
export const plainPct = (v) => (v === null || v === undefined || !isFinite(v) ? '—' : `${String(Math.round(v * 100) / 100).replace('.', ',')} %`)

// Сводка для шапки блока: текущие значения и разрыв «ставка − инфляция»
export function macroSummary(series, todayIso) {
  const at = (id) => (series && series[id] ? valueAt(series[id], todayIso) : null)
  const policy = at('policy_rate')
  const cpi = lastPoint(series && series.cpi_yoy)
  return {
    policy,
    overnightCD: at('overnight_cd'),
    cpi: cpi ? cpi.v : null,
    cpiMonth: cpi ? cpi.d.slice(0, 7) : '',
    core: lastPoint(series && series.core_yoy)?.v ?? null,
    realPolicy: policy !== null && cpi ? realRate(policy, cpi.v) : null
  }
}

export const MACRO_PAD = { l: 56, r: 16, t: 12, b: 36 } // поля графика (px во viewBox): по ним мышь переводится в данные
export const MACRO_MIN_SPAN = 0.5 // дальше приближать смысла нет: полгода
export const MACRO_MAX_SPAN = 60

const YEAR_MS = 365.25 * DAY
// Годы от сегодня (отрицательные — прошлое): та же ось X, что у графика роста, поэтому и общий движок (composables/useChartView.js)
export const yearsFrom = (iso, todayIso) => (ms(iso) - ms(todayIso)) / YEAR_MS
export const isoAt = (years, todayIso) => new Date(ms(todayIso) + years * YEAR_MS).toISOString().slice(0, 10)

const usable = (series, ids) => (ids || []).filter((id) => series && series[id] && series[id].length)

/** Окно по умолчанию: years лет назад (0 — с первой точки, весь период), до сегодня с небольшим запасом справа. Левее первой точки данных не уходит. */
export function macroWindow({ series, ids, todayIso, years = 0 }) {
  const use = usable(series, ids)
  if (!use.length) return { x0: -1, x1: 0.05 }
  const first = use.map((id) => series[id][0].d).sort()[0]
  const firstT = yearsFrom(first, todayIso)
  const left = years ? Math.max(-years, firstT) : firstT
  const span = Math.max(-left, MACRO_MIN_SPAN)
  return { x0: -span, x1: span * 0.02 }
}

/** Вертикаль: auto — под видимое окно без редких выбросов (один сбойный месяц не должен сплющивать график), ext — крайности по всем данным. */
export function macroRanges({ series, ids, view, todayIso, floor = 0, log = false }) {
  const visible = [], all = []
  for (const id of usable(series, ids)) {
    const pts = series[id]
    const before = valueAt(pts, isoAt(view.x0, todayIso))
    if (before !== null) visible.push(before)
    for (const p of pts) {
      all.push(p.v)
      const t = yearsFrom(p.d, todayIso)
      if (t >= view.x0 && t <= view.x1) visible.push(p.v)
    }
  }
  return robustRanges({ visible, all, floor, log })
}

/**
 * График: ставка ступенькой (держится до следующей смены, последнее значение тянется до сегодня), инфляция ломаной.
 * view — окно { x0, x1 } в годах от сегодня, yRange — { lo, hi }. Точки за краями остаются в линии, обрезает рамка графика.
 * Возвращает { lines:[{id,color,label,d}], yTicks, xTicks, todayX, zeroY, empty, pad, width, height }.
 */
export function buildMacroChart({ series, ids, view, yRange, todayIso, width = 1000, height = 220, pad = MACRO_PAD, scale = 'linear' }) {
  const use = usable(series, ids)
  if (!use.length) return { lines: [], yTicks: [], xTicks: [], todayX: 0, zeroY: null, empty: true, pad, width, height }
  const { x0, x1 } = view
  const { lo, hi } = yRange
  const X = (t) => pad.l + ((t - x0) / (x1 - x0)) * (width - pad.l - pad.r)
  const sc = scaleOf(scale)
  // yRange — в шкальном пространстве (при log это логарифмы); значения ≤ 0 на логарифмической оси не рисуются (разрыв линии)
  const Y = (v) => pad.t + (1 - (sc.fwd(v) - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const margin = (x1 - x0) * 0.05

  const lines = use.map((id) => {
    const meta = SERIES[id] || { label: id, color: '#94a3b8', step: false }
    // точки окна с запасом и одна точка перед ним (чтобы линия входила с левого края)
    const pts = []
    let prev = null
    for (const p of series[id]) {
      if (sc.mode === 'log' && !(p.v > 0)) continue
      const t = yearsFrom(p.d, todayIso)
      if (t < x0 - margin) { prev = { t: x0 - margin, v: p.v }; continue }
      if (t > x1 + margin) break
      if (prev) { pts.push(prev); prev = null }
      pts.push({ t, v: p.v })
    }
    if (prev) pts.push(prev)
    let d = ''
    pts.forEach((p, i) => {
      const x = X(p.t).toFixed(1), y = Y(p.v).toFixed(1)
      d += i === 0 ? `M${x} ${y}` : meta.step ? `H${x}V${y}` : `L${x} ${y}`
    })
    if (meta.step && pts.length) d += `H${X(Math.min(x1 + margin, 0)).toFixed(1)}`
    // градиент по значению: вектор от значения from до to в пикселях графика (userSpaceOnUse), поэтому цвет привязан к уровню, а не к положению на экране
    const g = meta.gradient ? { y1: Y(meta.gradient.from), y2: Y(meta.gradient.to), stops: meta.gradient.stops.map(([offset, color, opacity]) => ({ offset, color, opacity })) } : null
    return { id, color: meta.color, label: meta.label, d, gradient: g && isFinite(g.y1) && isFinite(g.y2) && g.y1 !== g.y2 ? g : null }
  })

  return {
    lines,
    yTicks: scaleTicks(sc.mode, lo, hi, 5).map((v) => ({ v, y: Y(v) })),
    xTicks: timeTicks(x0, x1, todayIso).map((t) => ({ x: X(t.t), label: t.label })),
    todayX: X(0),
    zeroY: sc.mode === 'linear' && lo < 0 && hi > 0 ? Y(0) : null,
    pad, width, height,
    empty: false
  }
}

// ── Экономика США: сигналы по рядам FRED (чистые функции; подсказки на экране, не советы).

/**
 * Правило Сам (Sahm): средняя безработица за 3 месяца минус её минимум за последние 12 месяцев (включая текущий).
 * Рост на 0,5 п. п. и больше исторически совпадал с началом рецессии. Ряд — помесячные точки по возрастанию; меньше 14 точек — null.
 * Возвращает { value, signal } (value в п. п.).
 */
export function sahmRule(points) {
  const v = (points || []).map((p) => p.v)
  if (v.length < 14) return null
  const avg = []
  for (let i = 2; i < v.length; i++) avg.push((v[i] + v[i - 1] + v[i - 2]) / 3)
  const last = avg[avg.length - 1]
  const min = Math.min(...avg.slice(-12))
  const value = Math.round((last - min) * 100) / 100
  return { value, signal: value >= 0.5 }
}

/** Два квартала подряд спада ВВП — «техническая рецессия». Ряд поквартальный; false, если меньше двух точек. */
export function technicalRecession(points) {
  const n = (points || []).length
  return n >= 2 && points[n - 1].v < 0 && points[n - 2].v < 0
}

/** Состояние кривой доходности (10 л. − 2 г., п. п.): отрицательная — «инвертирована» (исторически опережала рецессии). null — нет данных. */
export const curveState = (spread) => (spread === null || spread === undefined || !isFinite(spread) ? null : spread < 0 ? 'inverted' : spread < 0.5 ? 'flat' : 'normal')

/** Уровень VIX словами: до 20 спокойно, 20–30 тревожно, выше 30 паника. */
export const vixLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 20 ? 'calm' : v <= 30 ? 'nervous' : 'panic')

// ── Крипто: просадка от максимума, рост за период, индекс страха и жадности словами (чистые функции).

/** Просадка от исторического максимума, %: 0 на максимуме, −50 — цена вдвое ниже пика. Ряд по возрастанию дат. */
export function drawdown(points) {
  let peak = 0
  const out = []
  for (const p of points || []) {
    if (!(p.v > 0)) continue
    if (p.v > peak) peak = p.v
    out.push({ d: p.d, v: (p.v / peak - 1) * 100 })
  }
  return out
}

/** Максимум за всю историю и где цена сейчас относительно него: { ath, date, fromAth (%, ≤ 0) }. Нет данных — null. */
export function athInfo(points) {
  const pts = (points || []).filter((p) => p.v > 0)
  if (!pts.length) return null
  let top = pts[0]
  for (const p of pts) if (p.v > top.v) top = p
  const last = pts[pts.length - 1]
  return { ath: top.v, date: top.d, fromAth: (last.v / top.v - 1) * 100 }
}

/** Изменение цены за years лет назад (по последней точке не позже даты), % — не среднегодовое. Нет данных так далеко — null. */
export function changeSince(points, todayIso, years) {
  const last = lastPoint(points)
  if (!last || !(years > 0)) return null
  const then = valueAt(points, isoAt(-years, todayIso))
  return then > 0 && last.v > 0 ? (last.v / then - 1) * 100 : null
}

/** Индекс страха и жадности словами: до 25 крайний страх, до 45 страх, до 55 нейтрально, до 75 жадность, выше крайняя жадность. */
export const fearGreedLevel = (v) => (v === null || v === undefined || !isFinite(v) ? null : v < 25 ? 'extreme-fear' : v < 45 ? 'fear' : v <= 55 ? 'neutral' : v <= 75 ? 'greed' : 'extreme-greed')

// Кривая доходности Treasury: сроки слева направо и ряды, из которых берётся точка кривой (10 лет лежит в источнике «us»)
export const CURVE_TENORS = [
  { id: 'ust_1m', label: '1м' }, { id: 'ust_3m', label: '3м' }, { id: 'ust_6m', label: '6м' }, { id: 'ust_1y', label: '1г' },
  { id: 'ust_2y', label: '2г' }, { id: 'ust_3y', label: '3г' }, { id: 'ust_5y', label: '5л' }, { id: 'ust_7y', label: '7л' },
  { id: 'ust_10y', label: '10л' }, { id: 'ust_20y', label: '20л' }, { id: 'ust_30y', label: '30л' }
]

/** Снимок кривой на дату: по точке на срок. yearsAgo = 0 — последние значения, иначе значение на дату «столько лет назад» (последнее не позже неё). Нет данных — v: null. */
export function curveSnapshot(series, todayIso, yearsAgo = 0) {
  return CURVE_TENORS.map(({ id, label }) => {
    const pts = (series && series[id]) || []
    const v = yearsAgo > 0 ? valueAt(pts, isoAt(-yearsAgo, todayIso)) : (lastPoint(pts)?.v ?? null)
    return { id, label, v }
  })
}

// Шаг сетки «круглый»: 1, 2, 2.5 или 5 на степень десяти
function niceStep(raw) {
  const k = Math.pow(10, Math.floor(Math.log10(raw)))
  for (const m of [1, 2, 2.5, 5, 10]) if (m * k >= raw) return m * k
  return 10 * k
}

/**
 * График снимков кривой: сроки равномерно по горизонтали, доходность по вертикали. snaps — [{ key, color, points: curveSnapshot(...) }].
 * Возвращает { empty, xs: [{ x, label }], yTicks: [{ y, v }], lines: [{ key, color, d }], width, height, pad }. Пропуски (v: null) рвут линию.
 */
export function curveChart(snaps, { width = 1000, height = 220, pad = MACRO_PAD } = {}) {
  const vals = snaps.flatMap((s) => s.points.map((p) => p.v)).filter((v) => typeof v === 'number' && isFinite(v))
  const base = { width, height, pad, xs: [], yTicks: [], lines: [] }
  if (!vals.length) return { ...base, empty: true }
  let lo = Math.min(...vals), hi = Math.max(...vals)
  if (!(hi > lo)) { lo -= 0.5; hi += 0.5 }
  const margin = (hi - lo) * 0.08
  lo -= margin; hi += margin
  const step = niceStep((hi - lo) / 4)
  const yOf = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * (height - pad.t - pad.b)
  const n = snaps[0].points.length
  const xOf = (i) => pad.l + (n > 1 ? (i / (n - 1)) * (width - pad.l - pad.r) : 0)
  const yTicks = []
  for (let t = Math.ceil(lo / step) * step; t <= hi + 1e-9; t += step) yTicks.push({ y: yOf(t), v: Math.round(t * 1000) / 1000 })
  const lines = snaps.map((s) => {
    let d = '', pen = false
    s.points.forEach((p, i) => {
      if (typeof p.v !== 'number' || !isFinite(p.v)) { pen = false; return }
      d += `${pen ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(p.v).toFixed(1)} `
      pen = true
    })
    return { key: s.key, color: s.color, d: d.trim() }
  })
  return { ...base, empty: false, xs: snaps[0].points.map((p, i) => ({ x: xOf(i), label: p.label })), yTicks, lines }
}

/** Рост реальных зарплат: годовой рост заработка минус годовая инфляция на тот же месяц (п. п.). Нет пары — null. */
export function realWageGrowth(wagesYoY, cpiYoY) {
  const w = lastPoint(wagesYoY)
  if (!w) return null
  const c = valueAt(cpiYoY, w.d)
  return c === null ? null : { d: w.d, v: Math.round((w.v - c) * 100) / 100 }
}

/** Цель ФРС по инфляции PCE, % годовых */
export const FED_INFLATION_TARGET = 2

/** Млрд долларов словами крупно: 5900 → «5,9 трлн $», 480 → «480 млрд $». Нет значения — «—». */
export function billionsText(v) {
  if (v === null || v === undefined || !isFinite(v)) return '—'
  const a = Math.abs(v)
  return a >= 1000 ? `${String(Math.round((v / 1000) * 100) / 100).replace('.', ',')} трлн $` : `${String(Math.round(v)).replace('.', ',')} млрд $`
}

/** Спред высокодоходных облигаций к Treasury (п. п.) словами: до 4 — спокойно, до 6 — настороженно, выше — стресс. null — нет данных. */
export const creditLevel = (hy) => (hy === null || hy === undefined || !isFinite(hy) ? null : hy < 4 ? 'calm' : hy <= 6 ? 'wary' : 'stress')

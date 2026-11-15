import { lastPoint, valueAt, isoAt, realRate, sahmRule, technicalRecession, curveState, vixLevel, creditLevel, FED_INFLATION_TARGET } from './macro.js'

// Скоркард: всё считается на экране из рядов, которые уже есть в кеше. Это сводка и правила «если — то», а не прогноз и не торговый сигнал:
// каждый вывод показывает, из чего он получен.

const round = (v, d = 2) => Math.round(v * 10 ** d) / 10 ** d
const ok = (v) => typeof v === 'number' && isFinite(v)

// ── Реальные ставки и кэрри стран (по реестру стран с сервера: ставка и инфляция каждой страны)

/** Строки «реальная ставка» по странам: ставка центробанка, инфляция, реальная ставка (по Фишеру) и разница со ставкой США (кэрри), по убыванию реальной ставки. */
export function realRateRows(countries, series, today) {
  const us = (countries || []).find((c) => c.cc === 'us')
  const fed = us?.metrics?.policy ? valueAt(series?.[us.metrics.policy], today) : null
  return (countries || [])
    .filter((c) => c.metrics?.policy && c.metrics?.cpi)
    .map((c) => {
      const policy = valueAt(series?.[c.metrics.policy], today)
      const cpi = lastPoint(series?.[c.metrics.cpi])?.v ?? null
      const real = ok(policy) && ok(cpi) ? realRate(policy, cpi) : null
      return { cc: c.cc, name: c.name, policy, cpi, real, carry: ok(policy) && ok(fed) ? policy - fed : null }
    })
    .filter((r) => r.real !== null)
    .sort((a, b) => b.real - a.real)
}

// ── Индекс стресса: среднее z-оценок (на сколько «сигм» значение выше своей нормы за всю историю ряда)

export const STRESS_PARTS = [
  { id: 'us_vix', label: 'VIX (страх)', sign: 1 },
  { id: 'us_hy', label: 'Спред HY (кредитный риск)', sign: 1 },
  { id: 'us_nfci', label: 'Финансовые условия (NFCI)', sign: 1 },
  { id: 'us_spread', label: 'Кривая 10 л. − 2 г. (инверсия)', sign: -1 },
  { id: 'us_usd', label: 'Индекс доллара', sign: 1 }
]
export const STRESS_MIN_PARTS = 3

/** Среднее и разброс ряда; меньше 12 точек или нулевой разброс — null. */
export function meanStd(points) {
  const v = (points || []).map((p) => p.v).filter(ok)
  if (v.length < 12) return null
  const mean = v.reduce((a, b) => a + b, 0) / v.length
  const std = Math.sqrt(v.reduce((a, b) => a + (b - mean) ** 2, 0) / v.length)
  return std > 0 ? { mean, std } : null
}

/** Z-оценка значения ряда на дату (со знаком компонента: у кривой инверсия — это плюс к стрессу). Нет данных — null. */
export function zAt(points, stats, sign, iso) {
  const v = valueAt(points, iso)
  return v === null || !stats ? null : round((sign * (v - stats.mean)) / stats.std, 2)
}

/** Индекс стресса на дату: среднее z-оценок доступных компонентов (нужно хотя бы STRESS_MIN_PARTS). Возвращает { value, parts:[{id,label,z}] } или null. */
export function stressAt(series, iso, stats = statsOf(series)) {
  const parts = STRESS_PARTS.map((p) => ({ ...p, z: zAt(series?.[p.id], stats[p.id], p.sign, iso) })).filter((p) => p.z !== null)
  if (parts.length < STRESS_MIN_PARTS) return null
  return { value: round(parts.reduce((a, p) => a + p.z, 0) / parts.length), parts: parts.map(({ id, label, z }) => ({ id, label, z })) }
}
const statsOf = (series) => Object.fromEntries(STRESS_PARTS.map((p) => [p.id, meanStd(series?.[p.id])]))

/** Ряд индекса стресса по датам VIX (недельным): для графика. Точек нет, пока компонентов меньше минимума. */
export function stressSeries(series) {
  const stats = statsOf(series)
  const out = []
  for (const p of series?.us_vix || []) {
    const s = stressAt(series, p.d, stats)
    if (s) out.push({ d: p.d, v: s.value })
  }
  return out
}

/** Уровень стресса словами: ниже −0,5 спокойно, до 0,5 обычно, до 1,5 напряжённо, выше — стресс. */
export const stressLevel = (v) => (!ok(v) ? null : v < -0.5 ? 'calm' : v <= 0.5 ? 'normal' : v <= 1.5 ? 'tense' : 'stress')
export const STRESS_TEXT = { calm: 'спокойно', normal: 'обычно', tense: 'напряжённо', stress: 'стресс' }

// ── Опережающие сигналы: светофор по каждому показателю (ok / warn / alarm)

const ST = { ok: 'ok', warn: 'warn', alarm: 'alarm' }

/** Список сигналов: подпись, значение, состояние и пояснение. Нет данных у показателя — его нет в списке. */
export function signalRows(series, today) {
  const rows = []
  const add = (key, label, value, unit, state, hint) => { if (state) rows.push({ key, label, value, unit, state, hint }) }
  const spread = lastPoint(series?.us_spread)?.v ?? null
  const cs = curveState(spread)
  add('curve', 'Кривая 10 л. − 2 г.', spread, 'п. п.', cs && (cs === 'inverted' ? ST.alarm : cs === 'flat' ? ST.warn : ST.ok), 'Инверсия (ниже нуля) исторически опережала рецессии на год-полтора')
  const sahm = sahmRule(series?.us_unrate)
  add('sahm', 'Правило Сам', sahm?.value ?? null, 'п. п.', sahm && (sahm.signal ? ST.alarm : sahm.value >= 0.3 ? ST.warn : ST.ok), 'Рост безработицы на 0,5 п. п. над минимумом года совпадал с началом рецессии')
  const gdp = series?.us_gdp
  const gl = lastPoint(gdp)?.v ?? null
  add('gdp', 'Рост ВВП (квартал)', gl, '%', gl !== null && (technicalRecession(gdp) ? ST.alarm : gl < 0 ? ST.warn : ST.ok), 'Два квартала спада подряд — «техническая рецессия»')
  const hy = lastPoint(series?.us_hy)?.v ?? null
  const cl = creditLevel(hy)
  add('hy', 'Спред HY', hy, 'п. п.', cl && (cl === 'stress' ? ST.alarm : cl === 'wary' ? ST.warn : ST.ok), 'До 4 спокойно, до 6 настороженно, выше — кредитный стресс')
  const nf = lastPoint(series?.us_nfci)?.v ?? null
  add('nfci', 'Финансовые условия (NFCI)', nf, 'п.', nf !== null && (nf > 0.5 ? ST.alarm : nf > 0 ? ST.warn : ST.ok), 'Выше нуля — условия жёстче среднего')
  const vix = lastPoint(series?.us_vix)?.v ?? null
  const vl = vixLevel(vix)
  add('vix', 'VIX', vix, 'пт.', vl && (vl === 'panic' ? ST.alarm : vl === 'nervous' ? ST.warn : ST.ok), 'До 20 спокойно, 20–30 тревожно, выше 30 паника')
  const cl4 = claimsRise(series?.us_claims)
  add('claims', 'Заявки на пособие', cl4, '% к минимуму года', cl4 !== null && (cl4 >= 25 ? ST.alarm : cl4 >= 10 ? ST.warn : ST.ok), 'Среднее за 4 недели против минимума за год: рост на 25 % и больше — рынок труда слабеет')
  const sent = lastPoint(series?.us_sentiment)?.v ?? null
  add('sentiment', 'Настроения потребителей', sent, 'пт.', sent !== null && (sent < 60 ? ST.alarm : sent < 75 ? ST.warn : ST.ok), 'Индекс Мичигана: ниже 60 — подавленные настроения')
  return rows
}

/** Рост среднего заявок за 4 недели над минимумом такого среднего за год, %. Меньше 56 точек — null. */
export function claimsRise(points) {
  const v = (points || []).map((p) => p.v)
  if (v.length < 56) return null
  const avg = []
  for (let i = 3; i < v.length; i++) avg.push((v[i] + v[i - 1] + v[i - 2] + v[i - 3]) / 4)
  const last = avg[avg.length - 1]
  const min = Math.min(...avg.slice(-52))
  return min > 0 ? round((last / min - 1) * 100, 1) : null
}

/** Сводка светофора: сколько сигналов в каждом состоянии. */
export const signalTally = (rows) => ({ ok: rows.filter((r) => r.state === 'ok').length, warn: rows.filter((r) => r.state === 'warn').length, alarm: rows.filter((r) => r.state === 'alarm').length, total: rows.length })

// ── Режим экономики по правилам (два вопроса: растёт ли экономика и какая инфляция)

export const REGIMES = {
  recession: { label: 'Рецессия', note: 'экономика сжимается при невысокой инфляции' },
  stagflation: { label: 'Стагфляция', note: 'экономика сжимается, инфляция высокая' },
  overheating: { label: 'Перегрев', note: 'рост есть, инфляция выше цели' },
  disinflation: { label: 'Дезинфляция', note: 'рост слабый, инфляция снижается' },
  goldilocks: { label: 'Золотая середина', note: 'рост есть, инфляция около цели' }
}
export const INFLATION_HIGH = FED_INFLATION_TARGET + 1.5

/** Режим по правилам. Возвращает { key, label, note, why:[строки] } или null, если нет ни роста, ни инфляции. Объяснение — какие значения привели к выводу. */
export function regime(series, today) {
  const infl = lastPoint(series?.us_cpi_yoy)
  const gdp = lastPoint(series?.us_gdp)
  if (!infl || !gdp) return null
  const sahm = sahmRule(series?.us_unrate)
  const contracting = gdp.v < 0 || !!sahm?.signal
  const weak = !contracting && gdp.v < 1.5
  const high = infl.v >= INFLATION_HIGH
  const before = valueAt(series?.us_cpi_yoy, isoAt(-0.5, infl.d))
  const falling = before !== null && infl.v <= before - 0.5
  const why = [`ВВП за квартал ${fmt(gdp.v)} % годовых`, `инфляция ${fmt(infl.v)} % (цель ${FED_INFLATION_TARGET} %, «высокая» — от ${fmt(INFLATION_HIGH)} %)`]
  if (sahm) why.push(`правило Сам ${fmt(sahm.value)} п. п. — ${sahm.signal ? 'сигнал' : 'нет сигнала'}`)
  if (before !== null) why.push(`инфляция полгода назад ${fmt(before)} % → ${falling ? 'снижается' : 'не снижается'}`)
  const key = contracting ? (high ? 'stagflation' : 'recession') : high ? 'overheating' : weak && falling ? 'disinflation' : 'goldilocks'
  return { key, ...REGIMES[key], why }
}
const fmt = (v) => String(round(v, 1)).replace('.', ',')

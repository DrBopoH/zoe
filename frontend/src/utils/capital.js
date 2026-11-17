import { realRate } from './macro.js'
import { toUsd } from './dollar.js'

// Структура капитала и портфель ОВДП для Сметы. Чистые функции — без интерфейса.
// Капитал в долларовом эквиваленте (когда известен курс), иначе — только гривневые счета.

// Слои капитала, от «живых» денег к вложенным. Цвета по смыслу: доллар — зелёный, гривна (карты, ОВДП) — голубой и фиолетовый, крипто — оранжевый
export const LAYERS = [
  { key: 'cards', label: 'Карты и счета', color: '#60a5fa', hint: 'Деньги на картах и счетах банков' },
  { key: 'broker', label: 'Деньги на брокерском', color: '#93c5fd', hint: 'Свободные деньги на брокерском счёте ICU: ещё не вложены в облигации' },
  { key: 'bonds', label: 'ОВДП', color: '#a78bfa', hint: 'Стоимость облигаций на брокерском счёте (с накопленным купоном)' },
  { key: 'crypto', label: 'Крипто', color: '#fb923c', hint: 'Оценка кошельков Binance' }
]

const r2 = (v) => Math.round(v * 100) / 100 + 0

/**
 * Из чего состоит капитал: сумма по слоям и доля каждого. accounts — список счетов (с `portfolio` у брокерского и Binance).
 * Без rates — как раньше: только гривневые счета, в гривне. С rates ({ usdUah, eurUah }) — всё в долларовом эквиваленте:
 * доллар как есть, гривна и евро по курсу НБУ, Binance по курсу его же последней разведки (usdt_uah). Счета в других валютах пропускаются (skipped).
 * Возвращает { unit, total, totalUah, parts: [{ key, label, color, hint, value, pct }], currencies: [{ key, label, value, pct }], skipped } —
 * только ненулевые; pct — от итога (null при нулевом итоге); totalUah — приблизительный гривневый эквивалент (null без курса).
 */
export function capitalStructure(accounts, rates = null) {
  const usd = !!(rates && rates.usdUah)
  const sum = { cards: 0, broker: 0, bonds: 0, crypto: 0 }
  const cur = { UAH: 0, USD: 0, EUR: 0 }
  let skipped = 0
  const conv = (code, v) => {
    if (!usd) return code === 980 ? v : null
    return toUsd(code, v, rates)
  }
  const put = (layer, code, v) => {
    if (v === null) { skipped++; return }
    sum[layer] += v
    const k = code === 840 ? 'USD' : code === 978 ? 'EUR' : 'UAH'
    cur[k] += v
  }
  for (const a of accounts || []) {
    const bal = Number(a.balance) || 0
    const code = a.currency_code
    if (!usd && code !== 980) continue
    if (a.bank_key === 'binance') {
      const rate = a.portfolio && a.portfolio.usdt_uah > 0 ? a.portfolio.usdt_uah : null // баланс Binance хранится в гривне, по сути это доллары
      const v = usd ? (rate ? bal / rate : toUsd(980, bal, rates)) : bal
      if (v === null) { skipped++; continue }
      sum.crypto += v
      cur[usd ? 'USD' : 'UAH'] += v
    } else if (a.bank_key === 'icu') {
      const p = a.portfolio
      if (p && p.kind === 'bonds') { put('broker', code, conv(code, Number(p.cash) || 0)); put('bonds', code, conv(code, Number(p.invested) || 0)) } else put('broker', code, conv(code, bal))
    } else put('cards', code, conv(code, bal))
  }
  const total = r2(Object.values(sum).reduce((s, v) => s + v, 0))
  const share = (v) => (total > 0 ? Math.round((v / total) * 1000) / 10 : null)
  const parts = LAYERS.map((l) => ({ ...l, value: r2(sum[l.key]), pct: share(sum[l.key]) })).filter((p) => p.value !== 0)
  const currencies = usd ? CURRENCIES.map((c) => ({ ...c, value: r2(cur[c.key]), pct: share(cur[c.key]) })).filter((c) => c.value !== 0) : []
  return { unit: usd ? '$' : '₴', total, totalUah: usd ? r2(total * rates.usdUah) : total, parts, currencies, skipped }
}

// Валюты, в которых «исходно» лежат деньги (слоёный пирог: чем ближе к жизни, тем «живее» валюта)
export const CURRENCIES = [
  { key: 'UAH', label: 'Гривна', color: '#60a5fa' },
  { key: 'USD', label: 'Доллар', color: '#34d399' },
  { key: 'EUR', label: 'Евро', color: '#94a3b8' }
]

const DAY = 86400000
const yearsTo = (iso, todayIso) => Math.max(0, (Date.parse(`${iso}T00:00:00Z`) - Date.parse(`${todayIso}T00:00:00Z`)) / (365.25 * DAY))

/**
 * Портфель ОВДП по позициям брокерского журнала (/icu/ledger → positions: qty, value, coupon_rate, maturity, next_date, next_amount).
 * Ставка и срок — средние, взвешенные по стоимости позиции; без условий (нет ставки/погашения) и погашенные в расчёт не идут.
 * inflation — инфляция г/г, % (null — реальная доходность не считается).
 * Возвращает null, если облигаций с известной ставкой нет.
 */
export function portfolioStats(positions, todayIso, inflation = null) {
  let value = 0, rateW = 0, yearsW = 0, count = 0, skipped = 0
  const nextByDate = new Map()
  for (const p of positions || []) {
    const qty = Number(p.qty), v = Number(p.value), c = Number(p.coupon_rate)
    if (!(qty > 0)) continue
    if (p.next_date && p.next_date >= todayIso && Number(p.next_amount) > 0) nextByDate.set(p.next_date, (nextByDate.get(p.next_date) || 0) + Number(p.next_amount))
    if (p.no_terms || !(c > 0) || !p.maturity || p.maturity < todayIso || !(v > 0)) { skipped++; continue }
    value += v
    rateW += v * c
    yearsW += v * yearsTo(p.maturity, todayIso)
    count++
  }
  if (!count) return null
  const next = [...nextByDate.keys()].sort()[0]
  const rate = rateW / value
  return {
    count, skipped,
    value: r2(value),
    rate: Math.round(rate * 100) / 100,
    years: Math.round((yearsW / value) * 10) / 10,
    real: inflation === null ? null : realRate(rate, inflation),
    next: next ? { date: next, amount: r2(nextByDate.get(next)) } : null
  }
}

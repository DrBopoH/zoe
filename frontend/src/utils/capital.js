import { realRate } from './macro.js'

// Структура капитала и портфель ОВДП для Сметы. Чистые функции — без интерфейса.
// Капитал — гривневые счета (как плитка «Капитал» и график): валютные в него не входят.

// Слои капитала, от «живых» денег к вложенным; цвета — как у остальных блоков (зелёный, голубой, янтарный, фиолетовый)
export const LAYERS = [
  { key: 'cards', label: 'Карты и счета', color: '#34d399', hint: 'Деньги на картах и счетах банков' },
  { key: 'broker', label: 'Деньги на брокерском', color: '#38bdf8', hint: 'Свободные деньги на брокерском счёте ICU: ещё не вложены в облигации' },
  { key: 'bonds', label: 'ОВДП', color: '#fbbf24', hint: 'Стоимость облигаций на брокерском счёте (с накопленным купоном)' },
  { key: 'crypto', label: 'Крипто', color: '#a78bfa', hint: 'Оценка кошельков Binance в гривне' }
]

const r2 = (v) => Math.round(v * 100) / 100 + 0

/**
 * Из чего состоит капитал: сумма по слоям и доля каждого. accounts — список счетов (с `portfolio` у брокерского и Binance).
 * Возвращает { total, parts: [{ key, label, color, hint, value, pct }] } — только слои с ненулевой суммой; pct — от итога (null при нулевом итоге).
 */
export function capitalStructure(accounts) {
  const sum = { cards: 0, broker: 0, bonds: 0, crypto: 0 }
  for (const a of accounts || []) {
    if (a.currency_code !== 980) continue
    const bal = Number(a.balance) || 0
    if (a.bank_key === 'binance') sum.crypto += bal
    else if (a.bank_key === 'icu') {
      const p = a.portfolio
      if (p && p.kind === 'bonds') { sum.broker += Number(p.cash) || 0; sum.bonds += Number(p.invested) || 0 } else sum.broker += bal
    } else sum.cards += bal
  }
  const total = r2(Object.values(sum).reduce((s, v) => s + v, 0))
  const parts = LAYERS.map((l) => ({ ...l, value: r2(sum[l.key]), pct: total > 0 ? Math.round((sum[l.key] / total) * 1000) / 10 : null })).filter((p) => p.value !== 0)
  return { total, parts }
}

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

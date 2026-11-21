// Учёт дела «Glovo»: набор бейджей операций, итоги по ним и понедельные записи. Чистая логика без Vue.
// Бейджи (ключ уходит на бэкенд как есть): расходы и убытки — уход денег, три вида дохода — приход.
//   expense — Расход: сумма «конвертирована» (с некоторым спредом) в постоянный расходник;
//   loss    — Убыток: чистая потеря, деньги на ветер;
//   tips    — Чай: чаевые наличными, внесённые на сберегательную карту через банкомат (не облагаются налогом);
//   income  — Доход: налогооблагаемый приход на сберегательную карту;
//   payout  — Выплата: налогооблагаемая зарплата на счёт ФОП.
export const BADGES = [
  { key: 'expense', label: 'Расход', side: 'out', cls: 'bg-orange-500/15 text-orange-300 border-orange-500/30' },
  { key: 'loss', label: 'Убыток', side: 'out', cls: 'bg-rose-500/15 text-rose-300 border-rose-500/30' },
  { key: 'tips', label: 'Чай', side: 'in', cls: 'bg-amber-500/15 text-amber-300 border-amber-500/30' },
  { key: 'income', label: 'Доход', side: 'in', cls: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' },
  { key: 'payout', label: 'Выплата', side: 'in', cls: 'bg-sky-500/15 text-sky-300 border-sky-500/30' }
]

export const badgeOf = (key) => BADGES.find((b) => b.key === key) || null

const r2 = (v) => Math.round(v * 100) / 100 + 0

/**
 * Итоги по бейджам: по каждому ключу число операций и сумма (как в операции: приход плюс, расход минус).
 * marks — Map(id операции → ключ бейджа), transactions — операции кэша. Помеченная операция, которой нет в кэше, не считается.
 */
export function badgeTotals(transactions, marks) {
  const out = Object.fromEntries(BADGES.map((b) => [b.key, { count: 0, sum: 0 }]))
  for (const t of transactions || []) {
    const k = marks.get(t.id)
    if (!k || !out[k]) continue
    out[k].count += 1
    out[k].sum += Number(t.amount) || 0
  }
  for (const k of Object.keys(out)) out[k].sum = r2(out[k].sum)
  return out
}

/** Понедельник недели, в которую попадает дата YYYY-MM-DD (по календарю, без часовых поясов). Мусор → ''. */
export function weekMonday(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso || '')
  if (!m) return ''
  const d = new Date(Date.UTC(+m[1], +m[2] - 1, +m[3]))
  if (Number.isNaN(d.getTime()) || d.toISOString().slice(0, 10) !== iso) return ''
  const back = (d.getUTCDay() + 6) % 7 // пн=0 … вс=6
  d.setUTCDate(d.getUTCDate() - back)
  return d.toISOString().slice(0, 10)
}

/** Конец недели (воскресенье) по её понедельнику: для подписи «16.11 — 22.11». */
export function weekSunday(monday) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(monday || '')
  if (!m) return ''
  const d = new Date(Date.UTC(+m[1], +m[2] - 1, +m[3] + 6))
  return d.toISOString().slice(0, 10)
}

/** Сумма записей номинального дохода. */
export const nominalTotal = (weeks) => r2((weeks || []).reduce((s, w) => s + (Number(w.nominal) || 0), 0))

// ───────── Таймлайн номинального дохода ─────────
// Glovo фиксирует номинал за неделю (пн–вс), а деньги приходят на карту через 2–4 дня после отчёта, то есть на следующей неделе.
// Поэтому номинал недели N сверяется с приходом (бейджи «Доход» и «Выплата») в неделю N+1. «Чай» сюда не входит: это наличные, не часть выплаты Glovo.
export const PAYOUT_BADGES = ['income', 'payout']

const DAY = 86400000
const isoOf = (d) => d.toISOString().slice(0, 10)
const toMs = (iso) => Date.parse(`${iso}T00:00:00Z`)
export const addDays = (iso, n) => isoOf(new Date(toMs(iso) + n * DAY))
const daysBetween = (a, b) => Math.round((toMs(b) - toMs(a)) / DAY)

// Локальная календарная дата операции (YYYY-MM-DD): неделя считается так, как её видит человек, а не по UTC.
export function txDate(t) {
  const d = new Date(t.time)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/**
 * Недели таймлайна по возрастанию: от первой недели с данными (запись номинала или приход минус неделя) до текущей недели.
 * Лента не короче minWeeks недель (чтобы было куда вносить номинал) и продлевается назад на back недель по кнопке «Раньше».
 * Каждая неделя: { week, end, nominal (null — не внесён), received, count, firstDate, lagDays, delta, deltaPct, status }.
 *   received — приход (Доход + Выплата) за следующую неделю; lagDays — через сколько дней после воскресенья пришёл первый платёж;
 *   delta = received − nominal (минус — удержано налогами и вычетами); deltaPct — доля удержанного, % от номинала.
 *   status: 'none' — номинала нет; 'paid' — деньги пришли; 'pending' — ждём (неделя прихода ещё не кончилась); 'missing' — неделя прихода прошла, денег нет.
 */
export function timelineWeeks({ weeks, transactions, marks, today, minWeeks = 12, back = 0 }) {
  const nominalBy = new Map((weeks || []).map((w) => [w.week, Number(w.nominal) || 0]))
  const arrivals = [] // { date, amount }
  for (const t of transactions || []) {
    const k = marks.get(t.id)
    if (!PAYOUT_BADGES.includes(k)) continue
    const date = txDate(t)
    if (date) arrivals.push({ date, amount: Number(t.amount) || 0 })
  }
  const thisWeek = weekMonday(today)
  if (!thisWeek) return []
  const firsts = [...nominalBy.keys(), ...arrivals.map((a) => addDays(weekMonday(a.date), -7))].filter(Boolean).sort()
  const MAX_WEEKS = 260
  let start = addDays(thisWeek, -(Math.max(1, minWeeks) - 1) * 7)
  if (firsts.length && firsts[0] < start) start = firsts[0]
  start = addDays(start, -Math.max(0, back) * 7)
  if (daysBetween(start, thisWeek) / 7 > MAX_WEEKS) start = addDays(thisWeek, -MAX_WEEKS * 7)
  const out = []
  for (let w = start; w <= thisWeek; w = addDays(w, 7)) {
    const end = addDays(w, 6)
    const from = addDays(w, 7)
    const to = addDays(w, 13)
    const got = arrivals.filter((a) => a.date >= from && a.date <= to).sort((x, y) => (x.date < y.date ? -1 : 1))
    const received = r2(got.reduce((s, a) => s + a.amount, 0))
    const nominal = nominalBy.has(w) ? nominalBy.get(w) : null
    let status = 'none'
    if (nominal !== null) status = got.length ? 'paid' : today <= to ? 'pending' : 'missing'
    out.push({
      week: w,
      end,
      nominal,
      received,
      count: got.length,
      firstDate: got.length ? got[0].date : '',
      lagDays: got.length ? daysBetween(end, got[0].date) : null,
      delta: nominal !== null && got.length ? r2(received - nominal) : null,
      deltaPct: nominal > 0 && got.length ? r2(((nominal - received) / nominal) * 100) : null,
      status
    })
  }
  return out
}

/** Сводка по неделям, где номинал внесён и деньги пришли: номинал, пришло, удержано (в ₴ и % от номинала). */
export function timelineSummary(rows) {
  const paid = (rows || []).filter((r) => r.status === 'paid')
  const nominal = r2(paid.reduce((s, r) => s + r.nominal, 0))
  const received = r2(paid.reduce((s, r) => s + r.received, 0))
  const withheld = r2(nominal - received)
  return { weeks: paid.length, nominal, received, withheld, withheldPct: nominal > 0 ? r2((withheld / nominal) * 100) : null }
}

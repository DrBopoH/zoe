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

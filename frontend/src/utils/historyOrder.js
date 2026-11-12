// Порядок операций в истории. У записей брокерского отчёта (ICU) есть только дата — время в них заглушка (полдень), поэтому
// внутри дня их порядок задают логические правила, а не часы:
//   уход с банковского счёта → зачисление на брокерский → покупка → продажа → купон → погашение → вывод,
// и зачисление/вывод не могут стоять раньше/позже своей банковской половины перевода (там время точное).

const RANK = { deposit: 0, buy: 1, sell: 2, coupon: 3, redemption: 4, withdraw: 5 }
const STEP = 1000 // мс между соседними шагами цепочки

// запись брокерского журнала (дата без времени): зачисление/вывод из журнала и справочные строки облигаций
export const isDateOnly = (t) => (!!t.memo && !t.exact) || String(t.id).startsWith('ledger_')

const rankOf = (t) => (t.memo ? RANK[t.memo] ?? 4 : t.amount >= 0 ? RANK.deposit : RANK.withdraw)
const dayKey = (ms) => { const d = new Date(ms); return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}` }

// Map id → эффективное время (мс) для сортировки. Остальные операции — своё время.
export function effectiveTimes(transactions, memo = []) {
  const eff = new Map()
  const bankByAcc = new Map() // брокерский счёт → операции банков, ушедшие на него / пришедшие с него
  for (const t of transactions) {
    const ms = new Date(t.time).getTime()
    eff.set(t.id, ms)
    if (t.linked_account_id && !isDateOnly(t)) {
      if (!bankByAcc.has(t.linked_account_id)) bankByAcc.set(t.linked_account_id, [])
      bankByAcc.get(t.linked_account_id).push({ t, ms })
    }
  }
  for (const t of memo) eff.set(t.id, new Date(t.time).getTime()) // справочные строки с точным временем (переводы Binance) — как есть
  const groups = new Map()
  for (const t of [...transactions.filter(isDateOnly), ...memo.filter(isDateOnly)]) {
    const ms = new Date(t.time).getTime()
    const k = t.account_id + '|' + dayKey(ms)
    if (!groups.has(k)) groups.set(k, [])
    groups.get(k).push({ t, ms })
  }
  const used = new Set()
  for (const list of groups.values()) {
    list.sort((a, b) => rankOf(a.t) - rankOf(b.t) || a.ms - b.ms || String(a.t.id).localeCompare(String(b.t.id)))
    let prev = -Infinity
    for (const { t, ms } of list) {
      let at = ms
      if (t.id.startsWith('ledger_')) { // половина перевода на стороне банка: то же число с обратным знаком, в тот же день
        const half = (bankByAcc.get(t.account_id) || []).find(c => !used.has(c.t.id) && dayKey(c.ms) === dayKey(ms) && Math.abs(c.t.amount + t.amount) < 0.005)
        if (half) {
          used.add(half.t.id)
          at = t.amount >= 0 ? Math.max(at, half.ms + STEP) : Math.min(at, half.ms - STEP)
        }
      }
      at = Math.max(at, prev + STEP)
      eff.set(t.id, at)
      prev = at
    }
  }
  return eff
}

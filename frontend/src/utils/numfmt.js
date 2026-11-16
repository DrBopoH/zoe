// Единый вид чисел и дат во всём интерфейсе: десятичная запятая, пробел между разрядами, дата ДД.ММ.ГГГГ.
// Раньше каждый экран держал свою копию («String(Math.round(v * 100) / 100).replace('.', ',')» и т.п.) — теперь все ходят сюда.
const bad = (v) => v === null || v === undefined || typeof v === 'boolean' || !isFinite(v)

// Округлить до d знаков, лишние нули не писать: 9.40 → «9,4», 3 → «3». Нет значения — «—».
// Отличное от нуля число не округляется до «0», если просили знаки после запятой: 0.0007 при d = 2 даёт «0,0007» (две значащие цифры). При d = 0 — целое, как просили.
export const numText = (v, d = 2) => {
  if (bad(v)) return '—'
  const k = 10 ** d
  const r = Math.round(v * k) / k
  if (d > 0 && r === 0 && v !== 0) return Number(v).toFixed(Math.min(20, 1 - Math.floor(Math.log10(Math.abs(v))))).replace(/0+$/, '').replace('.', ',')
  return String(r).replace('.', ',')
}

// Ровно d знаков после запятой: 3 → «3,00».
export const fixedText = (v, d = 2) => (bad(v) ? '—' : Number(v).toFixed(d).replace('.', ','))

// Целое с пробелами между разрядами: 1234567.4 → «1 234 567».
export const intText = (v) => (bad(v) ? '—' : new Intl.NumberFormat('ru-RU').format(Math.round(v)))

// ГГГГ-ММ-ДД → ДД.ММ.ГГГГ; пусто — пустая строка (экран сам решает, писать ли «—»).
export const dmy = (iso) => (iso ? String(iso).split('-').reverse().join('.') : '')

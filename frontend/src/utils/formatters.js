export const formatMoney = (val) => {
  return new Intl.NumberFormat('uk-UA', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(val || 0)
}

export const toLocalDateStr = (d) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export const formatDateLong = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]))
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

export const formatShortDateName = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]))
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

export const formatMonthName = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, 1)
  return d.toLocaleDateString('ru-RU', { month: 'short' }).toUpperCase()
}

export const cleanAccountName = (name) => {
  if (!name) return ''
  return name.replace(/\s*\([^)]*\)/gi, '').trim()
}

export const cleanPersonName = (raw) => {
  if (!raw) return ''
  let s = raw.trim()
  s = s.replace(/^(від:|переказ\s+від:|переказ\s+на:|переказ:|для:|з\s+карти|на\s+карту)/i, '').trim()
  if (/^[\d\*\s]+$/.test(s)) return ''
  return s
}

export const isP2PTransaction = (t) => {
  if (t.counter_name && t.counter_name.trim() !== '') return true
  const desc = (t.description || '').toLowerCase()
  if (desc.includes('переказ') || desc.includes('від:') || desc.includes('з картки') || desc.includes('на картку')) {
    return true
  }
  const transferMCCs = [4829, 6538, 6536, 6537, 6012, 6011]
  return transferMCCs.includes(t.mcc)
}

export const formatMaskedIBAN = (iban) => {
  if (!iban || iban.length < 8) return iban
  return `${iban.slice(0, 4)}****${iban.slice(-4)}`
}

// Маскированный номер карты банка: 414951******5262 (начало + звёздочки + последние 4)
export const isMaskedPan = (s) => /[*•]/.test(String(s || ''))

export const validateAndCleanAccount = (input) => {
  if (!input) return null
  const compact = String(input).trim().replace(/[\s-]/g, '')
  if (/^\d{4,8}[*•xX]{2,12}\d{4}$/.test(compact)) return compact.replace(/[•xX]/g, '*')
  const clean = input.replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
  if (/^\d{12,19}$/.test(clean)) return clean
  if (/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(clean)) return clean
  return null
}
// Срок жизни счёта по выписке: «03.2024 — 11.2026». Даты банк не отдаёт — это первая/последняя операция.
const my = (unix) => {
  const d = new Date(unix * 1000)
  return `${String(d.getMonth() + 1).padStart(2, '0')}.${d.getFullYear()}`
}
export const lifeSpan = (acc) => {
  if (!acc?.opened_at && !acc?.closed_at) return ''
  const from = acc.opened_at ? my(acc.opened_at) : '?'
  const to = acc.is_closed && acc.closed_at ? my(acc.closed_at) : ''
  return to ? `${from} — ${to}` : `с ${from}`
}

// Даты для панели синка: «1 ноября 2026»
export const formatRuDate = (unix) =>
  unix ? new Date(unix * 1000).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }).replace(/\s?г\.$/, '') : ''

// «с 9 ноября по 10 ноября 2026» — без сокращений, год один раз, если он общий
export const formatRuRange = (from, to) => {
  const f = new Date(from * 1000)
  const t = new Date(to * 1000)
  const part = (d, withYear) => d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', ...(withYear ? { year: 'numeric' } : {}) }).replace(/\s?г\.$/, '')
  const sameYear = f.getFullYear() === t.getFullYear()
  return `с ${part(f, !sameYear)} по ${part(t, true)}`
}

export const plural = (n, one, few, many) => {
  const a = Math.abs(n) % 100
  const b = a % 10
  if (a > 10 && a < 20) return many
  if (b > 1 && b < 5) return few
  if (b === 1) return one
  return many
}

export const formatRuTime = (unix) =>
  unix ? new Date(unix * 1000).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : ''

// «через 12 мин», «через 1 ч 05 мин»
export const formatDuration = (sec) => {
  const s = Math.max(0, Math.round(sec))
  if (s < 60) return `${s} с`
  const m = Math.round(s / 60)
  if (m < 60) return `${m} мин`
  return `${Math.floor(m / 60)} ч ${String(m % 60).padStart(2, '0')} мин`
}

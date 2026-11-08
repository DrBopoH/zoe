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
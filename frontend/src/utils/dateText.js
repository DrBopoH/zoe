// Даты в полях вводим строкой ДД.ММ.ГГГГ (без календарного пикера). Внутри — ISO ГГГГ-ММ-ДД.
const pad = (n) => String(n).padStart(2, '0')

// 'ДД.ММ.ГГГГ' (также / - пробел как разделитель; ГГГГ-ММ-ДД тоже сойдёт) → 'ГГГГ-ММ-ДД' или null, если не дата
export const parseDateText = (text) => {
  const s = String(text || '').trim()
  let d, m, y
  let r = /^(\d{1,2})[.\/\-\s](\d{1,2})[.\/\-\s](\d{4})$/.exec(s)
  if (r) [, d, m, y] = r
  else if ((r = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s))) [, y, m, d] = r
  else return null
  d = +d; m = +m; y = +y
  const t = new Date(Date.UTC(y, m - 1, d))
  if (t.getUTCFullYear() !== y || t.getUTCMonth() !== m - 1 || t.getUTCDate() !== d) return null
  return `${y}-${pad(m)}-${pad(d)}`
}

// 'ГГГГ-ММ-ДД' → 'ДД.ММ.ГГГГ'
export const isoToText = (iso) => {
  const r = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(iso || ''))
  return r ? `${r[3]}.${r[2]}.${r[1]}` : ''
}

// Поле пустое или корректная дата
export const isDateTextOk = (text) => !String(text || '').trim() || parseDateText(text) !== null

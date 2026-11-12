// Автозаполнение формы облигации по ISIN из скачанного реестра (кеш рыночных данных, /market/bonds).
import { isoToText } from './dateText.js'

export const ISIN_LEN = 12

export const normIsin = (s) => String(s || '').trim().toUpperCase()

// Точное совпадение по ISIN (подстрока из q-поиска не годится: UA4000123 не равно UA4000123456)
export const findMarketBond = (list, isin) => {
  const id = normIsin(isin)
  return (Array.isArray(list) ? list : []).find((b) => normIsin(b.id) === id) || null
}

// Что подставить в форму: только пустые поля, чтобы не затереть набранное. Бескупонная (ставка 0) ставку не заполняет.
export const fillFromMarket = (form, bond) => {
  const out = { ...form }
  if (!String(out.name || '').trim() && bond.name) out.name = bond.name
  if (!String(out.maturity || '').trim() && bond.maturity) out.maturity = isoToText(bond.maturity)
  if (!(Number(out.coupon_rate) > 0) && Number(bond.coupon_rate) > 0) out.coupon_rate = bond.coupon_rate
  if (bond.nominal > 0 && (!out.nominal || out.nominal === 1000)) out.nominal = bond.nominal
  return out
}

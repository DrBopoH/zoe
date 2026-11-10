import { isPersonTransaction, counterpartyName, TRANSFER_MCC, sameId, isIbanLike, isPanLike } from './personLayout.js'
import { formatMaskedIBAN } from './formatters.js'

// Юрлица / магазины: торговый MCC, «ТОВ/ФОП…» в имени, возвраты. Нужны, чтобы отсечь шум во входящих.
const LEGAL_RE = /(^|[^a-zа-яіїєґ0-9])(тов|тзов|фоп|пп|пат|ат|llc|ltd|inc|ооо)([^a-zа-яіїєґ0-9]|$)|prom\.ua|повернення|возврат|refund|cashback|кешбек/i

export const isLegalEntity = (t) => {
  const mcc = Number(t?.mcc)
  if (mcc && !TRANSFER_MCC.has(mcc)) return true
  const text = `${t?.counter_name || ''} ${t?.description || ''}`
  return LEGAL_RE.test(text)
}

// Исходящие — только переводы контрагентам; входящие — всё, кроме юрлиц. all=true — без фильтра.
// банк как «контрагент» (PrivatBank, Monobank…) — это комиссия/внутренняя операция, а не человек
const BANK_BRAND_NAME = /^\s*(приватбанк|privatbank|private\s*bank|монобанк|monobank|mono|універсал\s*банк|universal\s*bank|(jsc|ат|пат|акб)\s+.*банк|райффайзен.*|raiffeisen.*|банк|bank|нбу|nbu)\s*$/i
export const isBankCounter = (t) => BANK_BRAND_NAME.test(String(t?.counter_name || '').trim())

export const passesFilter = (t, all = false) => {
  if (all) return true
  if (isBankCounter(t)) return false
  return t.amount < 0 ? isPersonTransaction(t) : !isLegalEntity(t)
}

const sameKind = (a, b) => isIbanLike(a) === isIbanLike(b)
// маска контрагента операции подходит карте?
export const matchesCard = (t, card) => {
  const c = String(t?.counter_iban || '')
  if (!c || !card) return null // у операции нет номера — сравнивать нечего
  return card.ids.some(id => sameKind(c, id) && sameId(c, id))
}

// Рядок истории: контрагент (из кэша алиасов) со счетами и состоянием «призначен / нет»
export const buildRow = (t, graph) => {
  const name = isPersonTransaction(t) && !isBankCounter(t) ? counterpartyName(t) : ''
  const node = name && graph ? graph.nodeFor(name) : null
  const eff = node?.effective || []
  const chips = eff.map(e => ({ key: e.key, label: formatMaskedIBAN(e.id), saved: !!e.persisted }))
  const assigned = eff.some(e => e.persisted)
  return {
    id: t.id,
    t,
    name,
    isCp: !!name,
    userId: node?.userId || '',
    chips,
    // маску отдельной строкой — только если имя не она сама («487407****44 · Monobank»)
    mask: (isIbanLike(t.counter_iban) || isPanLike(t.counter_iban)) && !name.includes(String(t.counter_iban).trim()) ? formatMaskedIBAN(t.counter_iban) : '',
    state: !name ? 'plain' : assigned ? 'green' : 'yellow'
  }
}

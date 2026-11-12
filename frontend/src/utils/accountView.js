import { formatMaskedIBAN } from './formatters.js'
import { pansOf, detectBankKey, isIbanLike, isPanLike } from './personLayout.js'
import { bankTheme } from './bankTheme.js'

// Счёт кэша → данные для <AccountPanel> (как раньше считала Операционная)
export const prepareAccount = (acc) => {
  const rawIban = acc.iban || acc.account_identifier || acc.name || ''
  const noIban = (acc.bank_key === 'icu' || acc.bank_key === 'binance') && !acc.iban
  const pans = pansOf(acc).filter(p => p !== rawIban)
  const theme = bankTheme(detectBankKey(rawIban, acc, pans), acc.type)
  return {
    ...acc,
    key: acc.id || acc.iban,
    // без идентификатора в алиасе файла имя счёта не показываем: как у любой карты без IBAN — «КАРТА»
    displayIban: noIban ? 'КАРТА' : formatMaskedIBAN(rawIban),
    pans: pans.map(formatMaskedIBAN),
    ids: [rawIban, ...pans].filter(x => x && (isIbanLike(x) || isPanLike(x))), // для сопоставления контрагента операций
    theme
  }
}

// Плитка из buildPersonLayout → те же данные. Карта без счёта в кэше: баланса нет (null), операций тоже.
export const cardFromTile = (t) => ({
  ...(t.cacheAcc || {}),
  key: t.key,
  id: t.cacheAcc?.id || null,
  balance: t.cacheAcc ? t.cacheAcc.balance : null,
  displayIban: t.iban ? formatMaskedIBAN(t.iban) : 'КАРТА',
  pans: (t.pans || []).map(formatMaskedIBAN),
  ids: [t.iban, ...(t.pans || [])].filter(Boolean), // все идентификаторы для сопоставления контрагента
  theme: bankTheme(t.bankKey, t.type)
})

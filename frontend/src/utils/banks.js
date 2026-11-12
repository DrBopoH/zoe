// Реестр банков: единственное место, где перечислены коды банков (МФО из IBAN), префиксы карт (BIN), названия и порядок.
// Нужен новый банк — одна запись здесь (плюс цветовая схема в bankTheme.js).
//   mfo    — коды банка в IBAN («UA44 322001 …»)
//   bin    — начала номеров карт, по которым банк узнаётся, когда IBAN неизвестен
//   letter — монограмма на значке, name — название на значке, label — короткое имя в подписях
//   order  — порядок групп на карточке Лица (у кого нет — после остальных)
export const BANKS = {
  mono: { mfo: ['322001'], bin: ['5375', '4441', '4874'], letter: 'M', name: 'monobank', label: 'Monobank', order: 1 },
  raif: { mfo: ['300335', '380805'], bin: [], letter: 'R', name: 'Райффайзен', label: 'Raiffeisen', order: 2 },
  privat: { mfo: ['305299'], bin: ['5168', '5457', '4149'], letter: 'П', name: 'ПриватБанк', label: 'PrivatBank', order: 3 },
  novapay: { mfo: ['935871'], bin: [], letter: 'N', name: 'NovaPay', label: 'NovaPay', order: 4 },
  icu: { mfo: ['380946'], bin: [], letter: 'I', name: 'ICU', label: 'ICU' }
}

const byMfo = new Map(Object.entries(BANKS).flatMap(([key, b]) => b.mfo.map((m) => [m, { key, ...b }])))

// банк по коду из IBAN (или undefined)
export const bankByMfo = (mfo) => byMfo.get(String(mfo))

// ключ банка по коду из IBAN ('' — неизвестен)
export const bankKeyByMfo = (mfo) => bankByMfo(mfo)?.key || ''

// ключ банка по номеру/началу карты ('' — неизвестен)
export const bankKeyByCard = (id) => {
  const s = String(id || '')
  for (const [key, b] of Object.entries(BANKS)) if (b.bin.some((p) => s.startsWith(p))) return key
  return ''
}

// порядок групп банков на карточке Лица
export const BANK_ORDER = [
  ...Object.entries(BANKS).filter(([, b]) => b.order).sort((a, b) => a[1].order - b[1].order).map(([k]) => k),
  'other'
]

// короткое имя банка для подписей (нет в реестре — '')
export const bankLabel = (key) => BANKS[key]?.label || ''

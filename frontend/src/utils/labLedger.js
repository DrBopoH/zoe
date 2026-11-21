// Бейджи страницы «ХимЛаб»: расход (весы, посуда, химматериал — к химматериалу потом цепляются карточки товаров), убыток (ошибки, потери),
// продажа (приход от покупателей). Ключи уходят на бэкенд как есть. Итоги и наличные записи общие со страницей Glovo (utils/glovoLedger.js).
export const LAB_BADGES = [
  { key: 'expense', label: 'Расход', side: 'out', cls: 'bg-orange-500/15 text-orange-300 border-orange-500/30' },
  { key: 'loss', label: 'Убыток', side: 'out', cls: 'bg-rose-500/15 text-rose-300 border-rose-500/30' },
  { key: 'sale', label: 'Продажа', side: 'in', cls: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' }
]

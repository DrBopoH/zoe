import { lastPoint, changeSince, athInfo } from './macro.js'

// Мировые рынки акций: индексы ОЭСР (помесячно, 2015 = 100) и оценка рынка США. Чистые функции, без интерфейса.
export const WORLD_INDICES = [
  { id: 'wm_us', label: 'США', group: 'developed' },
  { id: 'wm_jp', label: 'Япония', group: 'developed' },
  { id: 'wm_gb', label: 'Великобритания', group: 'developed' },
  { id: 'wm_de', label: 'Германия', group: 'developed' },
  { id: 'wm_fr', label: 'Франция', group: 'developed' },
  { id: 'wm_ch', label: 'Швейцария', group: 'developed' },
  { id: 'wm_ca', label: 'Канада', group: 'developed' },
  { id: 'wm_se', label: 'Швеция', group: 'other' },
  { id: 'wm_no', label: 'Норвегия', group: 'other' },
  { id: 'wm_pl', label: 'Польша', group: 'other' },
  { id: 'wm_cn', label: 'Китай', group: 'other' },
  { id: 'wm_kr', label: 'Южная Корея', group: 'other' },
  { id: 'wm_tw', label: 'Тайвань', group: 'other' },
  { id: 'wm_au', label: 'Австралия', group: 'other' },
  { id: 'wm_br', label: 'Бразилия', group: 'other' },
  { id: 'wm_sa', label: 'Саудовская Аравия', group: 'other' },
  { id: 'wm_ae', label: 'ОАЭ (Дубай)', group: 'other' },
  { id: 'wm_in', label: 'Индия', group: 'other' },
  { id: 'wm_tr', label: 'Турция (в лирах)', group: 'turkey' }
]

export const WORLD_GROUPS = [
  { key: 'developed', title: 'Развитые рынки', hint: 'Индексы ОЭСР, 2015 = 100: можно сравнивать между странами. Колесо: масштаб, перетаскивание: сдвиг' },
  { key: 'other', title: 'Остальные страны', hint: 'Индексы ОЭСР и Yahoo (Корея, Тайвань, Австралия, Бразилия, Саудовская Аравия, ОАЭ, Индия), 2015 = 100, в местной валюте' },
  { key: 'turkey', title: 'Турция (в лирах, шкала логарифмическая)', hint: 'Индекс в лирах: большую часть роста съела инфляция, поэтому шкала логарифмическая и отдельная', log: true }
]

export const worldIds = (group) => WORLD_INDICES.filter((i) => i.group === group).map((i) => i.id)

// Строки таблицы: последняя точка, изменение за год и удаление от максимума (по доступной истории), %
export function worldRows(series, today) {
  return WORLD_INDICES.map((i) => {
    const pts = series?.[i.id]
    const ath = athInfo(pts)
    return { id: i.id, label: i.label, last: lastPoint(pts), chg1y: changeSince(pts, today, 1), fromAth: ath ? ath.fromAth : null }
  })
}

// Оценка рынка США словами: индекс Баффета (стоимость акций / ВВП)
export function buffettLevel(v) {
  if (v === null || v === undefined || !isFinite(v)) return null
  return v < 90 ? 'дёшево' : v < 120 ? 'умеренно' : v < 160 ? 'дорого' : 'очень дорого'
}

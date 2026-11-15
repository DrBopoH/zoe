import { SOURCES } from './marketTable.js'

// Дерево разделов вкладки «Рынок»: группы раскрываются в шторке слева. Новый раздел = одна запись здесь
// (для ценных бумаг — ещё и в SOURCES из marketTable.js, для макро — своя панель в MarketView).
export const MACRO_SECTIONS = [
  { key: 'rates', label: 'Ставка НБУ', icon: 'rate', feed: 'macro' },
  { key: 'inflation', label: 'Инфляция', icon: 'inflation', feed: 'macro' },
  { key: 'fx', label: 'Курсы', icon: 'fx', feed: 'fx' },
  { key: 'countries', label: 'Страны', icon: 'macro', feed: 'ez' },
  { key: 'eurozone', label: 'Еврозона', icon: 'macro', feed: 'ez' },
  { key: 'fed', label: 'Ставка ФРС', icon: 'usa', feed: 'us' },
  { key: 'yields', label: 'Кривая Treasury', icon: 'usa', feed: 'uscurve' },
  { key: 'real', label: 'Реальные ставки', icon: 'usa', feed: 'usreal' },
  { key: 'prices', label: 'Цены США', icon: 'inflation', feed: 'usprices' },
  { key: 'jobs', label: 'Рынок труда США', icon: 'usa', feed: 'usjobs' },
  { key: 'activity', label: 'Активность США', icon: 'usa', feed: 'usact' },
  { key: 'liquidity', label: 'Ликвидность США', icon: 'usa', feed: 'usliq' },
  { key: 'credit', label: 'Кредит и доллар', icon: 'usa', feed: 'uscredit' },
  { key: 'usecon', label: 'Экономика США', icon: 'usa', feed: 'usecon' },
  { key: 'stocks', label: 'Биржа США', icon: 'stocks', feed: 'stocks' },
  { key: 'crypto', label: 'Крипто', icon: 'crypto', feed: 'crypto' }
]

export const MARKET_TREE = [
  { key: 'securities', label: 'Ценные бумаги', icon: 'bond', children: SOURCES.map(({ key, label, icon }) => ({ key, label, icon })) },
  { key: 'macro', label: 'Макро', icon: 'macro', children: MACRO_SECTIONS }
]

// Листья дерева: именно они выбираются и помнятся (старые сохранённые значения 'nbu' и 'ust' остаются верными)
export const leafKeys = (tree) => tree.flatMap((n) => (n.children ? n.children.map((c) => c.key) : [n.key]))
export const MARKET_SECTIONS = leafKeys(MARKET_TREE)

export const isBondSection = (key) => SOURCES.some((s) => s.key === key)
export const isMacroSection = (key) => MACRO_SECTIONS.some((s) => s.key === key)

// Какой источник опрашивает кнопка «Обновить» в разделе: бумаги — свой (nbu/ust), ставка и инфляция — общий macro, курсы — fx, ФРС — us, кривая — uscurve, реальные ставки — usreal, цены — usprices, труд — usjobs, активность — usact, ликвидность — usliq, кредит и доллар — uscredit, экономика США — usecon, еврозона и карточки стран — ez
export const feedOf = (key) => MACRO_SECTIONS.find((s) => s.key === key)?.feed ?? key

// Ключ группы, в которой лежит раздел (null — раздел не в группе)
export const groupOf = (tree, key) => tree.find((n) => n.children?.some((c) => c.key === key))?.key ?? null

import { SOURCES } from './marketTable.js'

// Дерево разделов вкладки «Рынок»: группы раскрываются в шторке слева. Новый раздел = одна запись здесь
// (для ценных бумаг — ещё и в SOURCES из marketTable.js, для макро — своя панель в MarketView).
export const MACRO_SECTIONS = [
  { key: 'rates', label: 'Ставка НБУ', icon: 'rate' },
  { key: 'inflation', label: 'Инфляция', icon: 'inflation' }
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

// Какой источник опрашивает кнопка «Обновить» в разделе: бумаги — свой (nbu/ust), макро — общий источник ставок и инфляции
export const feedOf = (key) => (isMacroSection(key) ? 'macro' : key)

// Ключ группы, в которой лежит раздел (null — раздел не в группе)
export const groupOf = (tree, key) => tree.find((n) => n.children?.some((c) => c.key === key))?.key ?? null

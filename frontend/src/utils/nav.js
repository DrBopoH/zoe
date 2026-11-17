// Навигация: верхние вкладки и разделы внутри «Учет». Старые сохранённые значения (до переезда
// «Операционной» и «Лиц» в «Учет») переводятся в новые, чтобы после обновления человек попал туда же.

export const TOP_TABS = ['dashboard', 'market', 'data']
// Страницы прежнего интерфейса живут в разделе «Архив» бокового меню (Build 3 — переборка дизайна); порядок и названия — отсюда и для меню, и для шапки
export const ARCHIVE_PAGES = [
  { key: 'dashboard', label: 'Смета' },
  { key: 'market', label: 'Рынок' },
  { key: 'data', label: 'Учет' }
]
export const archiveLabel = (key) => ARCHIVE_PAGES.find((p) => p.key === key)?.label || ''
export const DATA_SECTIONS = ['history', 'persons']

const LEGACY = {
  accounts: { tab: 'data', section: 'history' },
  entities: { tab: 'data', section: 'persons' }
}

export function resolveNav(storedTab, storedSection) {
  const legacy = LEGACY[storedTab]
  const tab = legacy ? legacy.tab : TOP_TABS.includes(storedTab) ? storedTab : 'dashboard'
  const section = legacy ? legacy.section : DATA_SECTIONS.includes(storedSection) ? storedSection : 'history'
  return { tab, section }
}

// Навигация: верхние вкладки и разделы внутри «Учет». Старые сохранённые значения (до переезда
// «Операционной» и «Лиц» в «Учет») переводятся в новые, чтобы после обновления человек попал туда же.

export const HOME_TAB = 'home' // главная: пустая страница до переборки дизайна (Build 3), вне архива
export const CASES_TAB = 'cases' // «Дела»: рабочие пространства пользователя; свой раздел бокового меню, не архив
export const TOP_TABS = [HOME_TAB, CASES_TAB, 'dashboard', 'market', 'data']
// Страницы прежнего интерфейса живут в разделе «Архив» бокового меню (Build 3 — переборка дизайна); порядок и названия — отсюда
export const ARCHIVE_PAGES = [
  { key: 'dashboard', label: 'Смета' },
  { key: 'market', label: 'Рынок' },
  { key: 'data', label: 'Учет' }
]
export const DATA_SECTIONS = ['history', 'persons']

const LEGACY = {
  accounts: { tab: 'data', section: 'history' },
  entities: { tab: 'data', section: 'persons' }
}

export function resolveNav(storedTab, storedSection) {
  const legacy = LEGACY[storedTab]
  const tab = legacy ? legacy.tab : TOP_TABS.includes(storedTab) ? storedTab : HOME_TAB
  const section = legacy ? legacy.section : DATA_SECTIONS.includes(storedSection) ? storedSection : 'history'
  return { tab, section }
}

// Навигация: верхние вкладки и разделы внутри «Учет». Старые сохранённые значения (до переезда
// «Операционной» и «Лиц» в «Учет») переводятся в новые, чтобы после обновления человек попал туда же.

export const TOP_TABS = ['dashboard', 'data']
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

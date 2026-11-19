import DebugSection from '../components/sections/DebugSection.vue'
import { ARCHIVE_PAGES } from './nav.js'

// Реестр разделов бокового меню. Новый раздел = одна запись здесь:
//   { key: 'уникальный-ключ', label: 'Название в меню', component: КомпонентПанели }
// Раздел-навигатор: вместо component — pages: [{ key, label }], клик по странице переключает вкладку приложения.
// Панель раздела собирается из <UiGroup title="ОБЩЕЕ"> … </UiGroup> (любое число абзацев).
// Панель может эмитить 'refresh' — App перезагрузит данные.
export const SECTIONS = [
  { key: 'archive', label: 'Архив', pages: ARCHIVE_PAGES },
  { key: 'debug', label: 'Управление', component: DebugSection }
]

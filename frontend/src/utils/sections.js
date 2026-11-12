import DebugSection from '../components/sections/DebugSection.vue'

// Реестр разделов бокового меню. Новый раздел = одна запись здесь:
//   { key: 'уникальный-ключ', label: 'Название в меню', component: КомпонентПанели }
// Панель раздела собирается из <UiGroup title="ОБЩЕЕ"> … </UiGroup> (любое число абзацев).
// Панель может эмитить 'refresh' — App перезагрузит данные.
export const SECTIONS = [
  { key: 'debug', label: 'Управление', component: DebugSection }
]

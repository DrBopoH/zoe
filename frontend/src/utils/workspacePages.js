// Реестр страниц дел: ключ из карточки (поле page) → страница. Дело хранит только ключ, а сама страница живёт здесь, на фронте.
// Новая страница = файл в views/workspaces/ + одна запись ниже; в окне создания она сразу появляется в списке «Страница».
// load — ленивая загрузка компонента (страница уходит в свой чанк и не утяжеляет остальные разделы). Страница получает prop workspace.
export const WORKSPACE_PAGES = [
  { key: 'glovo', label: 'Glovo (курьер)', load: () => import('../views/workspaces/GlovoPage.vue') },
  { key: 'lab', label: 'ХимЛаб (производство)', load: () => import('../views/workspaces/LabPage.vue') }
]

/** Запись реестра по ключу дела; null — ключа нет или страница не зарегистрирована (дело откроется с заглушкой). */
export const pageOf = (key) => WORKSPACE_PAGES.find((p) => p.key === key) || null

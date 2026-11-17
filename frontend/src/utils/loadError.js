// Ошибка загрузки раздела «Рынок» для плашки: сетевой сбой браузера («Failed to fetch») — это «нет связи с бэкендом»,
// остальное — текст самого бэкенда. Первая непустая из нескольких: у раздела бывает несколько источников (ряды + реестр стран).
export const loadErrorText = (msg) => {
  const m = String(msg || '').trim()
  if (!m) return ''
  if (/failed to fetch|networkerror|load failed|network request failed/i.test(m)) return 'Нет связи с бэкендом'
  return m
}

export const firstLoadError = (...errs) => {
  for (const e of errs) {
    const t = loadErrorText(e)
    if (t) return t
  }
  return ''
}

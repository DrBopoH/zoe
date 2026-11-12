import { ref, watch, nextTick } from 'vue'
import { storageGet, storageSet } from '../utils/storage.js'

// Выбранный раздел внутри вкладки: помнится в браузере, а у каждого раздела своя позиция прокрутки страницы
// (уходя запоминаем, возвращаясь — восстанавливаем; новый раздел — с верха).
export function useSection(storageKey, keys, fallback) {
  const saved = storageGet(storageKey)
  const section = ref(keys.includes(saved) ? saved : fallback)
  watch(section, (s) => storageSet(storageKey, s))

  const scrollOf = {}
  watch(section, (_, old) => { if (old) scrollOf[old] = window.scrollY }, { flush: 'pre' })
  watch(section, async (s) => {
    await nextTick()
    window.scrollTo(0, scrollOf[s] || 0)
  })
  return section
}

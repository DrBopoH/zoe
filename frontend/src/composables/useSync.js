import { ref } from 'vue'
import { storageGet, storageSet } from '../utils/storage.js'

// Состояние «Разведать», общее для кнопок во вкладке Смета и в окне «Выписки»: идёт ли обход (его выставляет App по
// /sync/status) и галочка «после обхода дописывать новые хвосты в источники» (помнится в браузере, по умолчанию включена).
export const isSyncing = ref(false)

const KEY = 'zoe.sync.autosave'
export const autoSave = ref(storageGet(KEY, '1') !== '0')
export const setAutoSave = (v) => { autoSave.value = !!v; storageSet(KEY, v ? '1' : '0') }

// Путь запуска обхода; save=1 — бэкенд сам запишет снятое в файлы-источники после обхода
export const syncPath = (auto) => (auto ? '/sync/full?save=1' : '/sync/full')

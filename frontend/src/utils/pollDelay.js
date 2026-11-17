// Как часто опрашивать лёгкий статус бэкенда (/sync/status).
// Идёт синк или опрос рынка — окно процесса должно двигаться ровно: каждые 2 с.
// В простое статус нужен только чтобы заметить запись в кеш (data_version) и пропавшую связь: хватает 5 с.
// Вкладка скрыта — опрос стоит, а при возвращении делается сразу (см. App.vue).
export const POLL_BUSY_MS = 2000
export const POLL_IDLE_MS = 5000
export const POLL_LOST_MS = 2000 // связи нет: проверяем чаще, чтобы предупреждение ушло, как только бэкенд вернулся

export function pollDelay({ syncing = false, marketRunning = false, lost = false } = {}) {
  if (syncing || marketRunning) return POLL_BUSY_MS
  return lost ? POLL_LOST_MS : POLL_IDLE_MS
}

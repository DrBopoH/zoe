// Безопасная обёртка над localStorage: в приватном окне, при очищенных или заблокированных данных он бросает —
// приложение должно работать и без него (тогда просто ничего не запоминаем).

export function storageGet(key, fallback = null) {
  try {
    const v = localStorage.getItem(key)
    return v === null ? fallback : v
  } catch {
    return fallback
  }
}

export function storageSet(key, value) {
  try {
    localStorage.setItem(key, value)
  } catch (err) {
    console.error(`Не удалось сохранить ${key}:`, err)
  }
}

export function loadJSON(key, fallback) {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : JSON.parse(raw)
  } catch {
    return fallback
  }
}

export const saveJSON = (key, value) => storageSet(key, JSON.stringify(value))

/**
 * Отметки свежести фидов: на какой версии кеша (dataVersion) фид читался в последний раз.
 * Раздел, который открыли до записи в кеш, при возврате перечитывается, а не показывает старые ряды; раздел, который нечем обновить, не перечитывается зря.
 * h — любой объект с series (ref) и load().
 */
export function feedStamps(getVersion) {
  const at = new Map()
  const load = (h) => { at.set(h, getVersion()); return h.load() }
  const stale = (h) => !Object.keys(h.series.value).length || at.get(h) !== getVersion()
  const ensure = (h) => (stale(h) ? load(h) : undefined)
  return { load, ensure, stale }
}

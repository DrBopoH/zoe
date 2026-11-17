import { SERIES } from './macro.js'
import { numText } from './numfmt.js'

// Страница «Источники»: здоровье источников и каталог рядов. Свежесть и частоту считает сервер (GET /api/macro/sources), здесь — подписи, поиск и CSV.

/** Состояние источника: error — последний опрос упал, empty — в кеше нет ни одного ряда, stale — есть устаревшие ряды, partial — часть обещанных рядов не пришла, ok. */
export function feedStatus(f) {
  if (f.error) return 'error'
  if (!f.cached) return 'empty'
  if (f.stale > 0) return 'stale'
  return f.cached < f.series ? 'partial' : 'ok'
}
export const STATUS_TEXT = { error: 'ошибка', empty: 'нет данных', stale: 'устарел', partial: 'неполный', ok: 'свежий' }

/** Сколько назад обновлялось: «только что», «5 мин назад», «3 ч назад», «2 дн. назад»; 0 — «нет». */
export function agoText(unix, nowMs = Date.now()) {
  if (!unix) return 'нет'
  const s = Math.max(0, Math.floor(nowMs / 1000 - unix))
  if (s < 60) return 'только что'
  if (s < 3600) return `${Math.floor(s / 60)} мин назад`
  if (s < 86400) return `${Math.floor(s / 3600)} ч назад`
  return `${Math.floor(s / 86400)} дн. назад`
}

/** Отставание последней точки: «сегодня», «3 дн.», «5 мес.». */
export function lagText(days) {
  if (!(days > 0)) return 'сегодня'
  if (days >= 365) return `${numText(days / 365, 1)} г.`
  return days >= 60 ? `${Math.round(days / 30)} мес.` : `${days} дн.`
}

/** Ряды для таблицы: к данным сервера добавлены подпись из справочника графиков и название источника. */
export function seriesRows(health) {
  const titles = Object.fromEntries((health?.feeds || []).map((f) => [f.id, f.title]))
  return (health?.series || []).map((s) => ({ ...s, label: SERIES[s.id]?.label || '', feedTitle: titles[s.feed] || '' }))
}

/** Поиск по id, подписи и названию источника (без регистра, все слова запроса должны найтись), фильтр по источнику и «только устаревшие». */
export function filterSeries(rows, { query = '', feed = '', staleOnly = false } = {}) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  return rows.filter((r) => {
    if (feed && r.feed !== feed) return false
    if (staleOnly && !r.stale) return false
    const hay = `${r.id} ${r.label} ${r.feedTitle}`.toLowerCase()
    return words.every((w) => hay.includes(w))
  })
}

/** CSV одного ряда: «date,value», точка как десятичный разделитель (открывается в Excel и pandas). */
export const seriesCsv = (points) => ['date,value', ...(points || []).map((p) => `${p.d},${p.v}`)].join('\n') + '\n'

/** Итог по источникам одной строкой для шапки. */
export function healthSummary(feeds) {
  const st = (feeds || []).map(feedStatus)
  return { total: st.length, ok: st.filter((s) => s === 'ok').length, stale: st.filter((s) => s === 'stale' || s === 'partial').length, error: st.filter((s) => s === 'error').length, empty: st.filter((s) => s === 'empty').length }
}

// Рыночные данные (ОВДП, US Treasury) — отдельный процесс рядом с обходом банков: свой итог в /sync/status → market.last
// и свои тосты, по одному на источник. Чистая функция: решает, что показать, не трогая интерфейс.

// seenAt — finished_at итога, который уже видели (null — ещё не видели ни одного ответа: итог до перезагрузки
// страницы не показываем). Возвращает новое seenAt и список тостов для pushToast.
const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1)
export const marketNotice = (seenAt, last) => {
  if (!last || !Array.isArray(last.feeds)) return { seenAt, toasts: [] }
  if (seenAt === null) return { seenAt: last.finished_at, toasts: [] }
  if (last.finished_at <= seenAt) return { seenAt, toasts: [] }
  const toasts = last.feeds.map((f) => f.state === 'error'
    ? { kind: 'err', title: `${f.title}: не обновилось`, lines: [f.error || 'ошибка источника'], ttl: 12000 }
    : { kind: 'ok', title: f.title, lines: [`${cap(f.unit || 'бумаг')} в кеше: ${f.fetched}`], ttl: 6000 })
  return { seenAt: last.finished_at, toasts }
}

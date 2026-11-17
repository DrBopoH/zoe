// Окно процесса рыночных данных (рядом с окном синка банков): висит, пока идёт опрос, исчезает после итога —
// зелёные тосты «ОВДП · НБУ — бумаг в кеше» остаются как раньше (utils/marketNotice.js).

// Показывать ли окно: опрос идёт и именно этот запуск (started_at) не закрыт крестиком.
export const marketPanelOpen = (market, dismissedAt) =>
  !!market && !!market.running && Array.isArray(market.feeds) && market.feeds.length > 0 && market.started_at !== dismissedAt

// Как рисовать строку источника: queued — пустой бар; run — бегущие полосы (процент неизвестен);
// pct — заливка по проценту; done — сплошной; error — красный.
export const feedView = (f) => {
  if (f.state === 'error') return { mode: 'error', pct: 0, text: f.error || 'ошибка источника' }
  if (f.state === 'done') return { mode: 'done', pct: 100, text: `${(f.unit || 'бумаг').replace(/^./, (c) => c.toUpperCase())}: ${f.fetched}` }
  if (f.state === 'queued') return { mode: 'queued', pct: 0, text: 'в очереди' }
  const pct = Math.min(100, Math.max(0, Math.round(f.pct || 0)))
  return { mode: pct > 0 ? 'pct' : 'run', pct, text: f.note || '' }
}

// Порядок строк в окне: готовые сверху (они схлопнуты в одну строку «Название ✓»), дальше ошибки и очередь,
// идущие — внизу, где виден их бар. Внутри группы порядок источников не меняется.
const feedRank = { done: 0, error: 1, queued: 2 }
export const orderFeeds = (feeds) =>
  feeds.map((f, i) => ({ f, i, r: feedRank[feedView(f).mode] ?? 3 })).sort((a, b) => a.r - b.r || a.i - b.i).map(x => x.f)

// Время с начала опроса «м:сс»
export const elapsedText = (startedAtSec, nowMs) => {
  const s = Math.max(0, Math.floor(nowMs / 1000 - startedAtSec))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}


// Чистая математика графика: таймфреймы, корзины времени, «красивые» шаги шкалы. Без Vue и DOM — покрыта tests/candleMath.test.mjs.

export const TFS = [
  { id: '1m', ms: 60_000, limit: 3000 },
  { id: '5m', ms: 300_000, limit: 2880 },
  { id: '15m', ms: 900_000, limit: 2880 },
  { id: '1h', ms: 3_600_000, limit: 2160 },
  { id: '4h', ms: 14_400_000, limit: 1500 },
  { id: '1D', day: true },
  { id: '1W', week: true }
]

export const dayStartOf = (ms) => { const d = new Date(ms); d.setHours(0, 0, 0, 0); return d.getTime() }
export const weekStartOf = (ms) => { const d = new Date(ms); d.setHours(0, 0, 0, 0); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return d.getTime() }
export const alignDown = (ms, step) => { const ds = dayStartOf(ms); return ds + Math.floor((ms - ds) / step) * step }

export const buildIntradayStarts = (stepMs, minT, want) => {
  const nowMs = Date.now()
  const out = []
  const d = new Date(nowMs)
  d.setHours(0, 0, 0, 0)
  let dayStart = d.getTime()
  while (out.length < want) {
    const next = new Date(dayStart)
    next.setDate(next.getDate() + 1)
    const k = Math.floor((Math.min(next.getTime() - 1, nowMs) - dayStart) / stepMs)
    for (let j = k; j >= 0 && out.length < want; j--) {
      const st = dayStart + j * stepMs
      out.push(st)
      if (st <= minT) return out.reverse() // корзина с самой ранней операцией — край истории
    }
    const prev = new Date(dayStart)
    prev.setDate(prev.getDate() - 1)
    prev.setHours(0, 0, 0, 0)
    dayStart = prev.getTime()
  }
  return out.reverse()
}
export const buildDailyStarts = (def, minT) => {
  const nowMs = Date.now()
  const out = []
  const first = def.day ? dayStartOf(minT) : weekStartOf(minT)
  const last = def.day ? dayStartOf(nowMs) : weekStartOf(nowMs)
  const c = new Date(first)
  while (c.getTime() <= last) {
    out.push(c.getTime())
    c.setDate(c.getDate() + (def.day ? 1 : 7))
  }
  return out
}


export const lowerBound = (arr, v) => { let lo = 0, hi = arr.length; while (lo < hi) { const m = (lo + hi) >> 1; if (arr[m] < v) lo = m + 1; else hi = m } return lo }

export const niceStep = (raw) => {
  const p = Math.pow(10, Math.floor(Math.log10(raw)))
  const f = raw / p
  return (f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10) * p
}

export const axisLabel = (v, step, lo, hi) => {
  const top = Math.max(Math.abs(lo), Math.abs(hi), Math.abs(v))
  const unit = top >= 1e6 ? 1e6 : top >= 1e4 || step >= 500 && top >= 1e3 ? 1e3 : 1
  const suffix = unit === 1e6 ? 'М' : unit === 1e3 ? 'к' : ''
  const u = step / unit
  let dec = 0 // знаков ровно столько, чтобы шаг шкалы читался точно (250 → 9,75к)
  while (dec < 3 && Math.abs(u * 10 ** dec - Math.round(u * 10 ** dec)) > 1e-6) dec++
  return (v / unit).toFixed(dec).replace('.', ',') + suffix
}

// Ход выбранной свечи для шапки истории: изменение капитала за период (закрытие − открытие) и размах внутри него
// (максимум − минимум, это волатильность «внутри дня», которую не видно в истории операций). Нет данных свечи → null.
// Процент считается от открытия; при нулевом открытии он не определён (null).
export function candleMove(c) {
  if (!c || ![c.open, c.close, c.high, c.low].every((v) => typeof v === 'number' && isFinite(v))) return null
  const r2 = (v) => Math.round(v * 100) / 100 + 0 // копейки без хвоста float и без «−0,00»
  const delta = r2(c.close - c.open)
  const swing = r2(c.high - c.low)
  const pct = (v) => (c.open > 0 ? Math.round((v / c.open) * 1000) / 10 + 0 : null)
  const bn = [c.bnOpen, c.bnClose].every((v) => typeof v === 'number' && isFinite(v)) ? r2(c.bnClose - c.bnOpen) : null
  return { open: c.open, close: c.close, high: c.high, low: c.low, delta, deltaPct: pct(delta), swing, swingPct: pct(swing), bnDelta: bn }
}

// ── Окно свечного графика на общем движке (composables/useChartView.js).
// Ось X — слоты (свечи и дни будущего), счёт от правого края данных: слот i (0…n−1) занимает [i − n, i − n + 1), слоты будущего — от 0 вправо.
// Когда слева подгружается история, координаты уже показанных свечей не меняются, окно стоит на месте.
export const CANDLE_PX = 22 // пикселей на свечу при масштабе по умолчанию
export const CANDLE_END_PAD = 1.4 // запас слотов у краёв (≈30 px)
export const CANDLE_MIN_SPAN = 8 // ближе свеча уже шире 90 px
export const CANDLE_MIN_PX = 1.5 // дальше свеча тоньше 2 px — смысла нет

/** Границы, за которые окно не уходит: от первой свечи до последнего слота будущего, с запасом у краёв. */
export const candleBounds = (n, future) => ({ min: -n - CANDLE_END_PAD, max: future + CANDLE_END_PAD })

/** Окно по умолчанию: свежий край. Со слотами будущего «сегодня» стоит на 70 % ширины (справа — выплаты), без них — у правого края. */
export function candleWindow({ n, future, plotW }) {
  const span = Math.max(CANDLE_MIN_SPAN, plotW / CANDLE_PX)
  const x1 = future > 0 ? span * 0.3 : CANDLE_END_PAD
  return { x0: x1 - span, x1 }
}

/** Размах окна: не ближе CANDLE_MIN_SPAN слотов, не дальше всей истории (и не тоньше CANDLE_MIN_PX на свечу). */
export function candleSpans({ n, future, plotW }) {
  const whole = n + future + 2 * CANDLE_END_PAD
  return { minSpan: CANDLE_MIN_SPAN, maxSpan: Math.max(CANDLE_MIN_SPAN, Math.min(whole, plotW / CANDLE_MIN_PX)) }
}

/** Индексы слотов [a, b), которые попадают в окно (с запасом в один слот с каждой стороны). */
export function visibleSlots(view, n, total) {
  const a = Math.max(0, Math.floor(view.x0 + n) - 1)
  return { a, b: Math.max(a, Math.min(total, Math.ceil(view.x1 + n) + 1)) }
}

/**
 * Диапазоны вертикали: auto — по свечам в окне, ext — по всем. S — { n, L, H, min, max } (низы и верхи свечей).
 * Отступ 4 % размаха; пустое окно (целиком в будущем) — общий диапазон.
 */
export function candleYRanges(S, view) {
  if (!S || !S.n) return { auto: { lo: 0, hi: 1 }, ext: { lo: 0, hi: 1 } }
  const make = (lo, hi) => {
    const pad = (hi - lo) * 0.04 || Math.abs(hi) * 0.01 || 1
    return { lo: lo - pad, hi: hi + pad }
  }
  const ext = make(S.min, S.max)
  const { a, b } = visibleSlots(view, S.n, S.n)
  let lo = Infinity, hi = -Infinity
  for (let i = a; i < b; i++) { if (S.L[i] < lo) lo = S.L[i]; if (S.H[i] > hi) hi = S.H[i] }
  return { auto: lo === Infinity ? ext : make(lo, hi), ext }
}

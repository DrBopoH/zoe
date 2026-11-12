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

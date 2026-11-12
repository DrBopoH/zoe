// Цикл Беннера (Сэмюэл Беннер, фермер из Огайо, «Prophecies of Future Ups and Downs in Prices», 1875): три вида лет.
// A — паника (обвал), B — хорошие времена, пик цен («продавать»), C — тяжёлые времена, дно цен («покупать»).
// ВАЖНО: сам Беннер прогнозировал только до 1904 года, остальное достроено после событий; предсказательной силы цикл не доказал.
// В программе это исторический слой-курьёз под графиком, а не сигнал. Списки — по Wikipedia (Benner's cycle).
export const BENNER = {
  A: { label: 'Паника', hint: 'год паники, обвал рынка', color: '#c98a96', years: [1783, 1803, 1819, 1837, 1857, 1873, 1891, 1911, 1927, 1945, 1965, 1981, 1999, 2019, 2035, 2053] },
  B: { label: 'Пик цен', hint: 'хорошие времена, высокие цены («продавать»)', color: '#c9b27a', years: [1783, 1791, 1800, 1810, 1818, 1827, 1837, 1845, 1854, 1864, 1872, 1881, 1891, 1899, 1908, 1918, 1926, 1935, 1945, 1953, 1962, 1972, 1980, 1989, 1999, 2007, 2016, 2026, 2034, 2043, 2053] },
  C: { label: 'Дно цен', hint: 'тяжёлые времена, низкие цены («покупать»)', color: '#7fb5a0', years: [1780, 1789, 1796, 1807, 1816, 1823, 1834, 1843, 1850, 1861, 1870, 1877, 1888, 1897, 1904, 1915, 1924, 1931, 1942, 1951, 1958, 1969, 1978, 1985, 1996, 2005, 2012, 2023, 2032, 2039, 2050, 2059] }
}

const KIND = new Map()
for (const k of ['C', 'B', 'A']) for (const y of BENNER[k].years) KIND.set(y, k) // год в двух списках (1945, 1999…) — побеждает паника

/** Вид года по циклу: 'A' | 'B' | 'C' или null (год вне списков). */
export const bennerKind = (year) => KIND.get(year) || null

const MARKED = [...KIND.keys()].sort((x, y) => x - y)

/** Фаза года: вид самого года (ключевой год) или вид последнего ключевого года до него (период между ними). null — до первого года таблицы. */
export function bennerPhase(year) {
  const own = KIND.get(year)
  if (own) return { kind: own, key: true }
  let lo = 0, hi = MARKED.length
  while (lo < hi) { const m = (lo + hi) >> 1; if (MARKED[m] < year) lo = m + 1; else hi = m }
  return lo ? { kind: KIND.get(MARKED[lo - 1]), key: false } : null
}

const yearOfMs = (ms) => new Date(ms).getUTCFullYear()

/**
 * Участки цикла в окне графика, на любой оси времени: xOf(ms) → px (ось может быть любой, лишь бы монотонной), fromMs/toMs — края окна,
 * left/right — границы поля графика в px. Ключевой год (A, B или C) — отдельный участок key:true, период между ключевыми годами —
 * общий участок key:false цвета предыдущей фазы (после B — «высокие цены», после C — «низкие», после A — «после паники»).
 * Возвращает [{ kind, key, from, to, x, w }], from/to — годы (to не включая).
 */
export function bennerSpans({ fromMs, toMs, xOf, left, right }) {
  if (!(toMs > fromMs)) return []
  const out = []
  for (let y = yearOfMs(fromMs); y <= yearOfMs(toMs); y++) {
    const ph = bennerPhase(y)
    if (!ph) continue
    const last = out[out.length - 1]
    if (!ph.key && last && !last.key && last.kind === ph.kind && last.to === y) last.to = y + 1
    else out.push({ kind: ph.kind, key: ph.key, from: y, to: y + 1 })
  }
  const res = []
  for (const sp of out) {
    const a = Math.max(left, xOf(Date.UTC(sp.from, 0, 1)))
    const b = Math.min(right, xOf(Date.UTC(sp.to, 0, 1)))
    if (b > a) res.push({ ...sp, x: a, w: b - a })
  }
  return res
}

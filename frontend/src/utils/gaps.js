// Пропуски в рядах данных: чистая математика, без Vue.
// Линия графика соединяет соседние точки прямой, а ступенька тянет значение до следующей точки, поэтому глазом «часы, в которые снимка не было»
// ничем не отличаются от обычного шага. Здесь — где ряд молчал заметно дольше, чем у него принято: такие отрезки графики рисуют пунктиром
// (свечи — штриховкой), а подсказка под курсором пишет «нет данных», а не последнее известное значение.
//
// «Принято» — это обычный шаг ряда рядом с точкой (медиана соседних интервалов), а не один шаг на весь ряд: у S&P 500 месячные данные до 2016 года
// и недельные после, у часовых снимков — час, у отчётов компаний — год. Свой шаг на каждый ряд вручную никто не задаёт.
const DAY = 86400000
const WINDOW = 8 // сколько интервалов с каждой стороны берётся для обычного шага
const GAP_K = 1.5 // интервал длиннее обычного в столько раз — пропуск (в месячном ряду выпавший месяц, в часовом — выпавший час)
const BUSINESS_MIN = DAY / 2 // обычный шаг от полусуток до двух суток — это торговые дни: выходные и праздничные длинные выходные пропуском не считаются
const BUSINESS_MAX = 2 * DAY
const BUSINESS_LIMIT = 5 * DAY

function median(a) {
  const s = Float64Array.from(a).sort()
  const m = s.length >> 1
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2
}

/** Самый длинный интервал (мс), который ещё считается обычным при обычном шаге expected. */
export function gapLimit(expected) {
  const k = expected * GAP_K
  return expected >= BUSINESS_MIN && expected < BUSINESS_MAX ? Math.max(k, BUSINESS_LIMIT) : k
}

/**
 * Пропуски ряда: t — время точек (мс) по возрастанию. Возвращает Uint8Array: 1 в позиции i — между точкой i−1 и точкой i ряд молчал слишком долго.
 * Обычный шаг у границы режимов (месяцы → недели) берётся по бо́льшей из двух сторон, чтобы сам стык не выглядел пропуском.
 * Короче трёх точек судить не по чему — пропусков нет.
 */
export function gapFlags(t) {
  const n = t.length
  const out = new Uint8Array(n)
  if (n < 3) return out
  const d = new Float64Array(n)
  for (let i = 1; i < n; i++) d[i] = t[i] - t[i - 1]
  const left = [], right = []
  for (let i = 1; i < n; i++) {
    left.length = 0
    right.length = 0
    for (let k = Math.max(1, i - WINDOW); k < i; k++) left.push(d[k])
    for (let k = i + 1; k < Math.min(n, i + 1 + WINDOW); k++) right.push(d[k])
    const expected = Math.max(left.length ? median(left) : 0, right.length ? median(right) : 0)
    if (expected > 0 && d[i] > gapLimit(expected)) out[i] = 1
  }
  return out
}

/**
 * Молчит ли ряд к моменту nowMs: с последней точки прошло заметно больше обычного шага (по последним интервалам).
 * Нужен ступенькам-отчётам: значение тянется до сегодня, но если новый отчёт давно должен был выйти, хвост — пунктир.
 */
export function tailGap(t, nowMs) {
  const n = t.length
  if (n < 2 || !(nowMs > t[n - 1])) return false
  const d = []
  for (let k = Math.max(1, n - WINDOW); k < n; k++) d.push(t[k] - t[k - 1])
  const expected = median(d)
  return expected > 0 && nowMs - t[n - 1] > gapLimit(expected)
}

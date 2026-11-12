// Общая математика интерактивных графиков (окно по времени, вертикаль, плавная подгонка). Без Vue и DOM.
// Окно — { x0, x1 } в единицах оси X графика (у графиков рынка это годы от сегодня), вертикаль — { lo, hi } в единицах значения.
// На этом стоит composables/useChartView.js; рост вложения (growthChart.js) и макро (macro.js) дают только свои данные и границы.

/** Окно после приближения: factor < 1 приближает, > 1 отдаляет; якорь (значение оси X) остаётся на месте; размах в пределах minSpan…maxSpan. */
export function zoomWindow(view, factor, anchor, { minSpan = 0, maxSpan = Infinity } = {}) {
  const span = Math.min(maxSpan, Math.max(minSpan, (view.x1 - view.x0) * factor))
  const ratio = (anchor - view.x0) / (view.x1 - view.x0)
  const x0 = anchor - ratio * span
  return { x0, x1: x0 + span }
}

/** Окно внутри границ { min, max } (единицы оси X): размах сохраняется, окно шире границ прижимается к левой. Границы null — без ограничений. */
export function clampWindow(view, bounds) {
  if (!bounds) return view
  const span = view.x1 - view.x0
  if (span >= bounds.max - bounds.min) return { x0: bounds.min, x1: bounds.min + span }
  const x0 = Math.min(Math.max(view.x0, bounds.min), bounds.max - span)
  return x0 === view.x0 ? view : { x0, x1: x0 + span }
}

/** Один шаг инерции после броска: v — скорость (px/мс), dt — мс. Возвращает { dx (px), v, done }; затухание 0,95 за кадр в 16 мс. */
export function inertiaStep(v, dt, stopBelow = 0.02) {
  const dx = v * dt
  const next = v * Math.pow(0.95, dt / 16)
  return { dx, v: next, done: Math.abs(next) <= stopBelow }
}

/** Сдвиг окна на dx. */
export const panWindow = (view, dx) => ({ x0: view.x0 + dx, x1: view.x1 + dx })

/** Ручной сдвиг по вертикали: на dv, не дальше крайностей ext. */
export function panY(range, dv, ext) {
  const span = range.hi - range.lo
  let lo = range.lo + dv
  lo = Math.max(ext.lo, Math.min(lo, ext.hi - span))
  return { lo, hi: lo + span }
}

/** Ручное приближение по вертикали: factor < 1 приближает, якорь (значение) остаётся на месте; размах от 2 % до 120 % крайностей. */
export function zoomY(range, factor, anchor, ext) {
  const full = ext.hi - ext.lo
  const span = Math.min(full * 1.2, Math.max(full * 0.02, (range.hi - range.lo) * factor))
  const ratio = (anchor - range.lo) / (range.hi - range.lo)
  const lo = anchor - ratio * span
  return { lo, hi: lo + span }
}

/** Один шаг плавного догона нарисованной вертикали до цели. Возвращает { range, done }: done — дошли (range равен цели). */
export function easeRange(shown, target, k = 0.2) {
  if (!shown) return { range: { ...target }, done: true }
  const span = target.hi - target.lo
  const lo = shown.lo + (target.lo - shown.lo) * k
  const hi = shown.hi + (target.hi - shown.hi) * k
  if (Math.abs(target.lo - lo) < span * 0.002 && Math.abs(target.hi - hi) < span * 0.002) return { range: { ...target }, done: true }
  return { range: { lo, hi }, done: false }
}

/** Значение с позиции p (0…1) среди отсортированных чисел (линейная интерполяция). */
export function quantile(sorted, p) {
  if (!sorted.length) return NaN
  const i = (sorted.length - 1) * Math.min(1, Math.max(0, p))
  const a = Math.floor(i), b = Math.ceil(i)
  return sorted[a] + (sorted[b] - sorted[a]) * (i - a)
}

/**
 * Диапазоны вертикали по значениям: auto — подгонка под видимое окно без редких выбросов (края по перцентилям, чтобы один
 * сбойный месяц не сплющивал весь график), ext — крайности по всем данным (ручной сдвиг и масштаб за них не уходят).
 * visible — значения в окне, all — все значения ряда. Отступ pad (доля размаха) с обеих сторон; floor — нижняя граница, если нужна (например 0).
 */
export function robustRanges({ visible, all, trim = 0.02, pad = 0.06, floor = null }) {
  const fin = (a) => a.filter((v) => typeof v === 'number' && isFinite(v)).sort((x, y) => x - y)
  const vis = fin(visible || []), every = fin(all || visible || [])
  const make = (lo, hi) => {
    if (!isFinite(lo) || !isFinite(hi)) { lo = 0; hi = 1 }
    if (floor !== null) lo = Math.min(lo, floor)
    if (!(hi > lo)) hi = lo + 1
    const p = (hi - lo) * pad
    return { lo: lo - p, hi: hi + p }
  }
  // выбросами считаем только хвост, если точек достаточно: на 10–20 точках срезать нечего
  const t = vis.length >= 30 ? trim : 0
  const auto = make(quantile(vis, t), quantile(vis, 1 - t))
  const ext = make(every[0] ?? NaN, every[every.length - 1] ?? NaN)
  // auto не шире ext
  return { auto: { lo: Math.max(auto.lo, ext.lo), hi: Math.min(auto.hi, ext.hi) }, ext }
}

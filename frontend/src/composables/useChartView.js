import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { zoomWindow, panWindow, panY, zoomY, easeRange, clampWindow, inertiaStep } from '../utils/chartView.js'

// Общий интерактив SVG-графиков: колесо — масштаб по времени (якорь под курсором), Shift+колесо — масштаб значения,
// перетаскивание вбок — сдвиг по времени, вверх/вниз — сдвиг значения (в пределах крайностей), двойной клик — сброс, наведение.
// Вертикаль гибридная: пока её не трогали руками, она плавно подтягивается под видимое окно; любое движение по времени
// возвращает подгонку, ручная вертикаль держится до следующего движения по времени.
//
// Что даёт график сам:
//   width, height, pad  — размеры viewBox и поля (число или функция), по ним мышь переводится в данные;
//   autoX()             — окно по умолчанию { x0, x1 };
//   ranges(view)        — { auto: {lo,hi}, ext: {lo,hi} }: подгонка под окно и крайности по всем данным;
//   spans               — { minSpan, maxSpan } размаха окна по X;
//   resetOn()           — массив, смена которого сбрасывает окно и вертикаль (другие фильтры — снова окно по умолчанию);
//   bounds()            — необязательно: { min, max } по X, за которые окно не уходит (по умолчанию границ нет);
//   inertia             — необязательно: после броска вбок окно доезжает по инерции.
// Возвращает: svgRef (повесить на <svg ref>), view, yShown (что рисовать), hoverX (данные под курсором или null),
// hoverPx (его x во viewBox), zoomed, setView, reset, tAt/vAt, eventT(e) (ось X под событием мыши) и обработчики мыши.
export function useChartView({ width, height, pad, autoX, ranges, spans = {}, resetOn = null, bounds = null, inertia = false }) {
  const W = () => (typeof width === 'function' ? width() : width)
  const H = () => (typeof height === 'function' ? height() : height)
  const P = () => (typeof pad === 'function' ? pad() : pad)

  const viewSet = ref(null) // null — окно по умолчанию (следует за данными), иначе { x0, x1 } после масштаба или сдвига
  const fit = (v) => (bounds ? clampWindow(v, bounds()) : v)
  const view = computed(() => viewSet.value || autoX())
  const manualY = ref(null) // null — вертикаль следует за окном, иначе { lo, hi } после ручного сдвига/масштаба
  const rng = computed(() => ranges(view.value))
  const target = computed(() => manualY.value || rng.value.auto)
  const yShown = ref(null) // то, что реально нарисовано: догоняет target плавно

  let raf = 0
  let dragY = false // идёт ручной сдвиг значения: подгонка без анимации (объявлено до watch(target), иначе TDZ в dev)
  const stopAnim = () => { if (raf) cancelAnimationFrame(raf); raf = 0 }
  const snap = () => { stopAnim(); yShown.value = { ...target.value } }
  const step = () => {
    const r = easeRange(yShown.value, target.value)
    if (r.done) return snap()
    yShown.value = r.range
    raf = requestAnimationFrame(step)
  }
  watch(target, () => { if (dragY || typeof requestAnimationFrame === 'undefined') snap(); else if (!raf) raf = requestAnimationFrame(step) }, { immediate: true })
  let inertiaRaf = 0
  const stopInertia = () => { if (inertiaRaf) cancelAnimationFrame(inertiaRaf); inertiaRaf = 0 }
  onBeforeUnmount(() => { stopAnim(); stopInertia() })
  if (resetOn) watch(resetOn, () => { viewSet.value = null; manualY.value = null })

  const svgRef = ref(null)
  // Пиксель страницы → единицы viewBox (график масштабируется по ширине)
  const px = (e) => { const r = svgRef.value.getBoundingClientRect(); return ((e.clientX - r.left) / r.width) * W() }
  const py = (e) => { const r = svgRef.value.getBoundingClientRect(); return ((e.clientY - r.top) / r.height) * H() }
  const plotW = () => W() - P().l - P().r
  const plotH = () => H() - P().t - P().b
  const cur = () => yShown.value || target.value
  // viewBox → данные: значение оси X под x и значение под y
  const tAt = (x) => view.value.x0 + ((x - P().l) / plotW()) * (view.value.x1 - view.value.x0)
  const vAt = (y) => { const r = cur(); return r.lo + (1 - (y - P().t) / plotH()) * (r.hi - r.lo) }

  const hoverX = ref(null)
  let pointerPx = null // последняя позиция мыши во viewBox: когда окно едет под неподвижной мышью (инерция, жест тачпада), подсказка следует за ней
  const refreshHover = () => { if (pointerPx !== null && hoverX.value !== null) hoverX.value = tAt(pointerPx) }
  const hoverPx = computed(() => (hoverX.value === null ? null : P().l + ((hoverX.value - view.value.x0) / (view.value.x1 - view.value.x0)) * plotW()))

  const onWheel = (e) => {
    stopInertia()
    // горизонтальный жест тачпада двигает окно по времени
    if (!e.shiftKey && Math.abs(e.deltaX) > Math.abs(e.deltaY)) {
      const r = svgRef.value.getBoundingClientRect()
      const dxUnits = (e.deltaX / r.width) * W() * ((view.value.x1 - view.value.x0) / plotW())
      viewSet.value = fit(panWindow(view.value, dxUnits))
      manualY.value = null
      refreshHover()
      return
    }
    const d = e.deltaY || e.deltaX
    const f = d < 0 ? 1 / 1.2 : 1.2
    if (e.shiftKey) { // масштаб значения вокруг точки под курсором
      manualY.value = zoomY(cur(), f, vAt(py(e)), rng.value.ext)
      return
    }
    viewSet.value = fit(zoomWindow(view.value, f, tAt(px(e)), spans))
    manualY.value = null
  }
  let drag = null
  let vel = 0, lastX = 0, lastT = 0 // скорость движения вбок (px/мс) для инерции
  const onDown = (e) => {
    stopInertia()
    drag = { x: px(e), y: py(e), view: view.value, range: { ...cur() }, axis: null }
    vel = 0; lastX = e.clientX; lastT = performance.now()
    svgRef.value.setPointerCapture(e.pointerId)
  }
  const onMove = (e) => {
    const x = px(e), y = py(e)
    pointerPx = x
    hoverX.value = tAt(x)
    if (!drag) return
    if (!drag.axis) { // ось выбирается по первому заметному движению
      const dx = Math.abs(x - drag.x), dy = Math.abs(y - drag.y)
      if (Math.max(dx, dy) < 4) return
      drag.axis = dy > dx ? 'y' : 'x'
    }
    if (drag.axis === 'y') {
      dragY = true
      const span = drag.range.hi - drag.range.lo
      manualY.value = panY(drag.range, ((y - drag.y) / plotH()) * span, rng.value.ext)
    } else {
      dragY = false
      viewSet.value = fit(panWindow(drag.view, ((drag.x - x) / plotW()) * (drag.view.x1 - drag.view.x0)))
      manualY.value = null // по X вертикаль плавно возвращается к подгонке
      const now = performance.now()
      if (now > lastT) vel = 0.8 * vel + 0.2 * ((lastX - e.clientX) / (now - lastT))
      lastX = e.clientX; lastT = now
    }
  }
  // бросок: окно доезжает по инерции, если рука остановилась не раньше чем за 80 мс до отпускания
  const startInertia = () => {
    let prev = performance.now(), v = vel
    const step = (t) => {
      const r = svgRef.value && svgRef.value.getBoundingClientRect()
      if (!r) return
      const s = inertiaStep(v, t - prev)
      prev = t; v = s.v
      const cur0 = view.value
      viewSet.value = fit(panWindow(cur0, ((s.dx / r.width) * W() / plotW()) * (cur0.x1 - cur0.x0)))
      refreshHover()
      inertiaRaf = s.done ? 0 : requestAnimationFrame(step)
    }
    inertiaRaf = requestAnimationFrame(step)
  }
  const onUp = () => {
    const flick = inertia && drag && drag.axis === 'x' && performance.now() - lastT < 80 && Math.abs(vel) > 0.02
    drag = null; dragY = false
    if (flick) startInertia()
  }
  const onLeave = () => { hoverX.value = null; drag = null; dragY = false }
  const reset = () => { stopInertia(); viewSet.value = null; manualY.value = null; hoverX.value = null } // под курсором теперь другая точка — подсказка вернётся при следующем движении
  const setView = (v) => { viewSet.value = v ? fit({ x0: v.x0, x1: v.x1 }) : null; manualY.value = null }
  const eventT = (e) => tAt(px(e)) // ось X под событием мыши (в том числе для клика без движения)
  const zoomed = computed(() => viewSet.value !== null || manualY.value !== null)

  return { svgRef, view, yShown, target, ranges: rng, hoverX, hoverPx, zoomed, setView, reset, tAt, vAt, eventT, onWheel, onDown, onMove, onUp, onLeave }
}

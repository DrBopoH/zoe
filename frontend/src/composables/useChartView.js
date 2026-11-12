import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { zoomWindow, panWindow, panY, zoomY, easeRange } from '../utils/chartView.js'

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
//   resetOn()           — массив, смена которого сбрасывает окно и вертикаль (другие фильтры — снова окно по умолчанию).
// Возвращает: svgRef (повесить на <svg ref>), view, yShown (что рисовать), hoverX (данные под курсором или null),
// hoverPx (его x во viewBox), zoomed, setView, reset, tAt/vAt и обработчики мыши.
export function useChartView({ width, height, pad, autoX, ranges, spans = {}, resetOn = null }) {
  const W = () => (typeof width === 'function' ? width() : width)
  const H = () => (typeof height === 'function' ? height() : height)
  const P = () => (typeof pad === 'function' ? pad() : pad)

  const viewSet = ref(null) // null — окно по умолчанию (следует за данными), иначе { x0, x1 } после масштаба или сдвига
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
  onBeforeUnmount(stopAnim)
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
  const hoverPx = computed(() => (hoverX.value === null ? null : P().l + ((hoverX.value - view.value.x0) / (view.value.x1 - view.value.x0)) * plotW()))

  const onWheel = (e) => {
    const d = e.deltaY || e.deltaX
    const f = d < 0 ? 1 / 1.2 : 1.2
    if (e.shiftKey) { // масштаб значения вокруг точки под курсором
      manualY.value = zoomY(cur(), f, vAt(py(e)), rng.value.ext)
      return
    }
    viewSet.value = zoomWindow(view.value, f, tAt(px(e)), spans)
    manualY.value = null
  }
  let drag = null
  const onDown = (e) => { drag = { x: px(e), y: py(e), view: view.value, range: { ...cur() }, axis: null }; svgRef.value.setPointerCapture(e.pointerId) }
  const onMove = (e) => {
    const x = px(e), y = py(e)
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
      viewSet.value = panWindow(drag.view, ((drag.x - x) / plotW()) * (drag.view.x1 - drag.view.x0))
      manualY.value = null // по X вертикаль плавно возвращается к подгонке
    }
  }
  const onUp = () => { drag = null; dragY = false }
  const onLeave = () => { hoverX.value = null; drag = null; dragY = false }
  const reset = () => { viewSet.value = null; manualY.value = null }
  const setView = (v) => { viewSet.value = v ? { x0: v.x0, x1: v.x1 } : null; manualY.value = null }
  const zoomed = computed(() => viewSet.value !== null || manualY.value !== null)

  return { svgRef, view, yShown, target, ranges: rng, hoverX, hoverPx, zoomed, setView, reset, tAt, vAt, onWheel, onDown, onMove, onUp, onLeave }
}

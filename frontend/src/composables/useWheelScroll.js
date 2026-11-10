import { onMounted, onBeforeUnmount } from 'vue'

// Колесо везде, где есть слайдер или горизонтальная прокрутка:
//  • над горизонтально прокручиваемой областью вертикальное колесо двигает её вбок
//    (дошли до края — событие уходит странице, чтобы не «залипать»);
//  • над <input type="range"> колесо крутит значение;
//  • внутри модалки ([data-modal]) колесо никогда не уходит странице под блюром: крутится список внутри, а на краю — ничего.
// Область с data-wheel="own" (свечной график) сама решает, что значит колесо, — её не трогаем.
const LINE = 16

const canScrollX = (el) => {
  const st = getComputedStyle(el)
  return /(auto|scroll)/.test(st.overflowX) && el.scrollWidth > el.clientWidth + 1
}
const canScrollY = (el) => {
  const st = getComputedStyle(el)
  return /(auto|scroll)/.test(st.overflowY) && el.scrollHeight > el.clientHeight + 1
}

const px = (e, el) => (e.deltaMode === 1 ? e.deltaY * LINE : e.deltaMode === 2 ? e.deltaY * el.clientWidth : e.deltaY)

const onWheel = (e) => {
  if (e.ctrlKey || !e.deltaY) return
  // горизонтальный жест (тачпад) браузер и так обработает сам
  if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) return

  let el = e.target instanceof Element ? e.target : null
  const modal = el?.closest('[data-modal]') || null // внутри модалки страница под блюром не скроллится никогда

  // слайдер
  const range = el?.closest('input[type="range"]')
  if (range && !range.disabled) {
    const step = Number(range.step) || 1
    const dir = e.deltaY < 0 ? 1 : -1
    const next = Math.min(Number(range.max), Math.max(Number(range.min), Number(range.value) + dir * step))
    if (next !== Number(range.value)) {
      range.value = String(next)
      range.dispatchEvent(new Event('input', { bubbles: true }))
      range.dispatchEvent(new Event('change', { bubbles: true }))
    }
    e.preventDefault()
    return
  }

  const stop = modal ? modal.parentElement : document.body
  for (; el && el !== stop; el = el.parentElement) {
    if (el.dataset?.wheel === 'own') return

    // вертикальный список: пока есть куда крутить — родной скролл; на краю в модалке не отдаём колесо странице
    if (canScrollY(el)) {
      const max = el.scrollHeight - el.clientHeight
      const atEdge = (e.deltaY < 0 && el.scrollTop <= 0) || (e.deltaY > 0 && el.scrollTop >= max - 1)
      if (!atEdge) return
      if (modal) e.preventDefault()
      return
    }

    if (!canScrollX(el)) continue
    const d = px(e, el)
    const max = el.scrollWidth - el.clientWidth
    const atEdge = (d < 0 && el.scrollLeft <= 0) || (d > 0 && el.scrollLeft >= max - 1)
    if (atEdge) break // край: страница (вне модалки) крутится дальше
    el.scrollLeft += d
    e.preventDefault()
    return
  }

  // колесо над модалкой, которому нечего крутить внутри, под блюром страницу не двигает
  if (modal) e.preventDefault()
}

export function useWheelScroll() {
  onMounted(() => document.addEventListener('wheel', onWheel, { passive: false, capture: true }))
  onBeforeUnmount(() => document.removeEventListener('wheel', onWheel, { capture: true }))
}

import { ref, watch, onUnmounted } from 'vue'

// Автопрокрутка страницы при перетаскивании (HTML5 drag&drop сам этого не делает, а шапка прибита сверху).
//  • зона у верхнего края начинается ПОД шапкой, а сама шапка — максимальная скорость
//  • скорость растёт плавно (smoothstep) с приближением к краю и догоняется с инерцией — без рывков
//  • edge.top / edge.bottom (0..1) — близость к краю, для подсветки зон
const ZONE = 150       // высота зоны разгона, px
const MAX_SPEED = 1500 // px/с на самом краю
const INERTIA = 9      // чем больше, тем быстрее скорость догоняет цель

const smooth = (k) => k * k * (3 - 2 * k)

export const useDragAutoScroll = (isDragging) => {
  const edge = ref({ top: 0, bottom: 0, headerBottom: 0 })
  let y = -1
  let vel = 0 // -1..1
  let raf = null
  let last = 0

  const headerBottom = () => document.querySelector('.app-bar')?.getBoundingClientRect().bottom || 0
  const onOver = (e) => { y = e.clientY }

  const proximity = () => {
    if (y < 0) return { top: 0, bottom: 0 }
    const hb = headerBottom()
    const topEdge = hb + ZONE
    const botEdge = window.innerHeight - ZONE
    return {
      top: y < topEdge ? smooth(Math.min(1, (topEdge - y) / ZONE)) : 0,
      bottom: y > botEdge ? smooth(Math.min(1, (y - botEdge) / ZONE)) : 0
    }
  }

  const tick = (t) => {
    const dt = Math.min(0.05, (t - last) / 1000 || 0.016)
    last = t
    const p = proximity()
    const target = p.bottom - p.top
    vel += (target - vel) * Math.min(1, dt * INERTIA)
    if (Math.abs(vel) > 0.01) window.scrollBy(0, vel * MAX_SPEED * dt)
    edge.value = { ...p, headerBottom: headerBottom() }
    raf = requestAnimationFrame(tick)
  }

  const start = () => {
    document.addEventListener('dragover', onOver, true)
    last = performance.now()
    raf = requestAnimationFrame(tick)
  }
  const stop = () => {
    document.removeEventListener('dragover', onOver, true)
    cancelAnimationFrame(raf)
    raf = null
    y = -1
    vel = 0
    edge.value = { top: 0, bottom: 0, headerBottom: 0 }
  }

  watch(isDragging, (on) => (on ? start() : stop()))
  onUnmounted(stop)

  return { edge }
}

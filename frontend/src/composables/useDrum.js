import { ref, computed, watch, onBeforeUnmount } from 'vue'

// Состояние «барабана»: дробная позиция pos плавно догоняет целевой индекс target.
// Колесо — в обе стороны (один шаг за ~60px прокрутки, так что и мышь, и тачпад ведут себя ровно).
export const useDrum = (count, initial = 0) => {
  const target = ref(initial)
  const pos = ref(initial)
  let raf = 0
  let acc = 0
  let accTimer = 0

  const clamp = (i) => Math.max(0, Math.min(Math.max(0, count.value - 1), i))
  const tick = () => {
    const d = target.value - pos.value
    if (Math.abs(d) < 0.002) {
      pos.value = target.value
      raf = 0
      return
    }
    pos.value += d * 0.2
    raf = requestAnimationFrame(tick)
  }
  const kick = () => { if (!raf) raf = requestAnimationFrame(tick) }

  const go = (i) => { target.value = clamp(i); kick() }
  const step = (d) => go(target.value + d)

  const STEP_PX = 60
  const onWheel = (e) => {
    e.preventDefault()
    acc += e.deltaY
    clearTimeout(accTimer)
    accTimer = setTimeout(() => { acc = 0 }, 200)
    while (Math.abs(acc) >= STEP_PX) {
      const d = Math.sign(acc)
      acc -= d * STEP_PX
      step(d)
    }
  }

  watch(count, () => go(target.value))
  onBeforeUnmount(() => { cancelAnimationFrame(raf); clearTimeout(accTimer) })

  return { pos, target, active: computed(() => target.value), go, step, onWheel }
}

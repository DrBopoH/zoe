<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import UiIcon from './ui/UiIcon.vue'

// Кнопки «в начало» (сверху по центру) и «в конец» (снизу по центру).
// Появляются, пока страница скроллится, и висят ещё 3 секунды после остановки.
const HIDE_AFTER_MS = 3000
const ANIM_MS = 350

const visible = ref(false)
const canUp = ref(false)
const canDown = ref(false)
const dir = ref('') // 'up' | 'down' — куда сейчас крутят; показываем только соответствующую кнопку
let lastY = window.scrollY

let hideTimer = null
let rafId = null
let animating = false

const measure = () => {
  const el = document.documentElement
  const max = el.scrollHeight - window.innerHeight
  canUp.value = window.scrollY > 8
  canDown.value = window.scrollY < max - 8
}

const onScroll = () => {
  measure()
  const y = window.scrollY
  if (Math.abs(y - lastY) >= 2) { dir.value = y > lastY ? 'down' : 'up'; lastY = y }
  if (!canUp.value && !canDown.value) {
    visible.value = false
    return
  }
  visible.value = true
  clearTimeout(hideTimer)
  hideTimer = setTimeout(() => { visible.value = false }, HIDE_AFTER_MS)
}

// Быстрая прокрутка с затуханием: ≈ 0,35 с независимо от длины страницы
const jumpTo = (target) => {
  cancelAnimationFrame(rafId)
  const from = window.scrollY
  const start = performance.now()
  animating = true
  const step = (now) => {
    const k = Math.min(1, (now - start) / ANIM_MS)
    const ease = 1 - Math.pow(1 - k, 3)
    window.scrollTo(0, from + (target - from) * ease)
    if (k < 1) rafId = requestAnimationFrame(step)
    else animating = false
  }
  rafId = requestAnimationFrame(step)
}

const toTop = () => jumpTo(0)
const toBottom = () => jumpTo(document.documentElement.scrollHeight - window.innerHeight)

// пользователь сам крутанул колесо во время анимации — не мешаем ему
const cancelAnim = () => {
  if (animating) {
    cancelAnimationFrame(rafId)
    animating = false
  }
}

onMounted(() => {
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('wheel', cancelAnim, { passive: true })
  window.addEventListener('touchstart', cancelAnim, { passive: true })
})
onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('wheel', cancelAnim)
  window.removeEventListener('touchstart', cancelAnim)
  clearTimeout(hideTimer)
  cancelAnimationFrame(rafId)
})
</script>

<template>
  <Transition name="jump">
    <button v-if="visible && canUp && dir === 'up'" type="button" class="ui-jump ui-jump--up" aria-label="В начало" @click="toTop">
      <UiIcon name="arrowUp" size="lg" />
    </button>
  </Transition>
  <Transition name="jump">
    <button v-if="visible && canDown && dir === 'down'" type="button" class="ui-jump ui-jump--down" aria-label="В конец" @click="toBottom">
      <UiIcon name="arrowDown" size="lg" />
    </button>
  </Transition>
</template>
<script setup>
import { toRef } from 'vue'
import { useDrum } from '../composables/useDrum.js'
import AccountPanel from './AccountPanel.vue'

// Вертикальный «барабан» карт: центральная — обычная, дальше — меньше и прозрачнее,
// дальняя заходит под ближнюю. Карты за лицевой полуокружностью цилиндра (≥ 90°) полностью прозрачны.
const props = defineProps({
  cards: { type: Array, default: () => [] },
  modelValue: { type: Number, default: 0 }
})
const emit = defineEmits(['update:modelValue'])

const STEP_DEG = 34 // угол между соседними картами на цилиндре
const RADIUS = 250  // радиус цилиндра, px

const { pos, target, go, onWheel } = useDrum(toRef(() => props.cards.length), props.modelValue)

const select = (i) => {
  go(i)
  emit('update:modelValue', i)
}
const onWheelAndSync = (e) => {
  onWheel(e)
  if (target.value !== props.modelValue) emit('update:modelValue', target.value)
}

const style = (i) => {
  const deg = (i - pos.value) * STEP_DEG
  const a = Math.abs(deg)
  if (a >= 90) return { opacity: 0, visibility: 'hidden', pointerEvents: 'none' }
  const rad = (deg * Math.PI) / 180
  const depth = Math.cos(rad)
  const y = RADIUS * Math.sin(rad)
  const scale = 0.6 + 0.4 * depth
  return {
    transform: `translateY(calc(-50% + ${y.toFixed(1)}px)) rotateX(${(-deg * 0.35).toFixed(1)}deg) scale(${scale.toFixed(3)})`,
    opacity: Math.pow(depth, 1.4).toFixed(3),
    zIndex: 100 - Math.round(a)
  }
}
</script>

<template>
  <div class="drum" @wheel="onWheelAndSync">
    <div
      v-for="(c, i) in cards"
      :key="c.key"
      class="drum-item"
      :style="style(i)"
      role="option"
      :aria-selected="i === target"
      @click="select(i)"
    >
      <AccountPanel :card="c" :selected="i === target" />
    </div>
  </div>
</template>

<style scoped>
.drum {
  position: relative;
  height: 520px;
  perspective: 900px;
  overscroll-behavior: contain;
}
.drum-item {
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  cursor: pointer;
  will-change: transform, opacity;
}
</style>

<script setup>
import { computed } from 'vue'
import { BENNER, bennerSpans } from '../utils/benner.js'

// Слой цикла Беннера для любого графика с осью времени. Две части (layer): `bg` — колонки на всю высоту поля за данными
// (ключевой год едва заметной колонкой, период между ключевыми годами без заливки) и `strip` — тонкая полоса с буквами A / B / C в подвале.
// График даёт только xOf(ms) → px и края окна в мс; так слой работает и на макро-графиках, и на росте вложения, и на свечах.
const props = defineProps({
  layer: { type: String, default: 'strip' }, // bg | strip
  fromMs: { type: Number, required: true },
  toMs: { type: Number, required: true },
  xOf: { type: Function, required: true },
  left: { type: Number, required: true },
  right: { type: Number, required: true },
  top: { type: Number, default: 0 }, // bg: верх и низ поля графика
  bottom: { type: Number, default: 0 },
  stripY: { type: Number, default: 0 }, // strip: верх полосы
  stripH: { type: Number, default: 6 }
})
const spans = computed(() => bennerSpans({ fromMs: props.fromMs, toMs: props.toMs, xOf: props.xOf, left: props.left, right: props.right }))
const tip = (s) => (s.key ? `${s.from}: ${BENNER[s.kind].label} (${BENNER[s.kind].hint})` : `${s.from}–${s.to - 1}: после ${s.kind === 'A' ? 'паники' : s.kind === 'B' ? 'пика цен' : 'дна цен'}, ${BENNER[s.kind].hint}`)
</script>

<template>
  <g v-if="layer === 'bg'" pointer-events="none">
    <rect v-for="s in spans" :key="'bg' + s.from + s.key" :x="s.x" :y="top" :width="s.w" :height="bottom - top" :fill="BENNER[s.kind].color" :fill-opacity="s.key ? 0.07 : 0" />
  </g>
  <g v-else>
    <title>Цикл Беннера (1875): A паника, B пик цен, C дно цен. Плотный — сам год, бледный — период после него. Исторический курьёз, не сигнал</title>
    <g v-for="s in spans" :key="'st' + s.from + s.key">
      <rect :x="s.x" :y="stripY" :width="s.w" :height="stripH" :fill="BENNER[s.kind].color" :fill-opacity="s.key ? 0.75 : 0.16"><title>{{ tip(s) }}</title></rect>
      <text v-if="s.key && s.w > 12" :x="s.x + s.w / 2" :y="stripY + stripH - 1.1" text-anchor="middle" font-size="5.5" font-weight="700" fill="#0f172a" pointer-events="none">{{ s.kind }}</text>
    </g>
  </g>
</template>

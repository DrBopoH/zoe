<script setup>
import { computed } from 'vue'
import { formatRuDate } from '../utils/formatters.js'
import { summaryLayers } from '../utils/statements.js'

// Сводный таймлайн покрытия: ось всегда от самой старой даты до сегодня, рисуются выбранные счета (нет выбора — все).
// Заливки счетов смешиваются прозрачностью 1/n, штриховка ложится поверх них.
const props = defineProps({
  accounts: { type: Array, default: () => [] }, // все счета
  drawn: { type: Array, default: () => [] } // рисуемые счета
})

const layers = computed(() => summaryLayers(props.accounts, props.drawn, Math.floor(Date.now() / 1000)))

const fillStyle = (r) => ({ left: r.left + '%', width: r.width + '%', background: r.color, opacity: r.alpha })
const hatchStyle = (r) => ({
  left: r.left + '%',
  width: r.width + '%',
  opacity: r.alpha,
  background: `repeating-linear-gradient(135deg, rgb(226 232 240 / 0.55) 0 1px, transparent 1px 5px), color-mix(in srgb, ${r.color} 22%, transparent)`
})
</script>

<template>
  <div v-if="layers.to" class="space-y-0.5" data-testid="cover-summary">
    <div class="ui-tl">
      <span v-for="(r, i) in layers.fills" :key="'f' + i" class="ui-tl__run" :style="fillStyle(r)"></span>
      <span v-for="(r, i) in layers.hatches" :key="'h' + i" class="ui-tl__run" :style="hatchStyle(r)"></span>
    </div>
    <div class="flex items-center justify-between text-[9px] text-slate-500">
      <span>{{ formatRuDate(layers.from) }}</span>
      <span>{{ drawn.length === accounts.length ? 'все счета' : `выбрано ${drawn.length} из ${accounts.length}` }}</span>
      <span>{{ formatRuDate(layers.to) }}</span>
    </div>
  </div>
</template>

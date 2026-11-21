<script setup>
import { computed } from 'vue'
import { formatMoney } from '../utils/formatters.js'

// Каскад: приход растёт столбцами вверх, уходы режут сумму вниз, последний столбец — итог от нуля.
// steps: [{ key, label, v, color, total? }] — v со знаком; у итога (total) значение само задаёт высоту столбца.
const props = defineProps({ steps: { type: Array, default: () => [] } })

const H = 200
const cols = computed(() => {
  let run = 0
  const raw = props.steps.map((s) => {
    const from = s.total ? 0 : run
    const to = s.total ? s.v : run + s.v
    if (!s.total) run = to
    else run = s.v
    return { ...s, lo: Math.min(from, to), hi: Math.max(from, to) }
  })
  const top = Math.max(1, ...raw.map((r) => r.hi), 0)
  const bot = Math.min(0, ...raw.map((r) => r.lo))
  const span = top - bot
  const y = (v) => ((top - v) / span) * 100
  return {
    zero: y(0),
    items: raw.map((r) => ({ ...r, y: y(r.hi), h: Math.max(0.8, ((r.hi - r.lo) / span) * 100) }))
  }
})
const sign = (s) => (s.total ? '' : s.v > 0 ? '+' : s.v < 0 ? '−' : '')
const val = (s) => `${sign(s)}${formatMoney(s.total ? s.v : Math.abs(s.v))}`
</script>

<template>
  <div class="relative" data-testid="waterfall">
    <div class="relative flex gap-2 sm:gap-3" :style="{ height: H + 'px' }">
      <div class="absolute inset-x-0 border-t border-dashed border-slate-700 pointer-events-none" :style="{ top: cols.zero + '%' }" />
      <div v-for="c in cols.items" :key="c.key" class="relative flex-1 min-w-0" :data-col="c.key">
        <div class="absolute inset-x-1 sm:inset-x-2 rounded-sm transition-all" :style="{ top: c.y + '%', height: c.h + '%', background: c.color, opacity: c.total ? 1 : 0.85 }" />
      </div>
    </div>
    <div class="flex gap-2 sm:gap-3 mt-2">
      <div v-for="c in cols.items" :key="c.key" class="flex-1 min-w-0 text-center">
        <div class="text-[11px] sm:text-xs font-bold tabular-nums truncate" :class="c.total ? (c.v < 0 ? 'text-rose-400' : 'text-emerald-400') : 'text-slate-200'">{{ val(c) }}</div>
        <div class="text-[9px] sm:text-[10px] uppercase tracking-wider text-slate-500 truncate" :title="c.label">{{ c.label }}</div>
      </div>
    </div>
  </div>
</template>

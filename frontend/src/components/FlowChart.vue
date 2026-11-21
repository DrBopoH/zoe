<script setup>
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { BADGES } from '../utils/glovoLedger.js'
import { formatMoney } from '../utils/formatters.js'

// Деньги дела по неделям: вверх приход (Чай, Доход, Выплата), вниз уход (Расход, Убыток), линия — накопленный результат.
// flow: [{ week, expense, loss, tips, income, payout, net, cum }] — строки estimateModel().flow
const props = defineProps({ flow: { type: Array, default: () => [] } })

const COLORS = { expense: '#f97316', loss: '#f43f5e', tips: '#f59e0b', income: '#34d399', payout: '#38bdf8' }
const UP = ['income', 'payout', 'tips']
const DOWN = ['expense', 'loss']
const MINCOL = 34, MAXCOL = 72, PADL = 8, PADT = 14, H = 220, PADB = 22

// Колонки растягиваются на всю ширину блока, но не уже MINCOL (тогда блок листается вбок)
const box = ref(null)
const avail = ref(0)
let ro
onMounted(() => {
  avail.value = box.value?.clientWidth || 0
  if (typeof ResizeObserver !== 'undefined' && box.value) { ro = new ResizeObserver(() => { avail.value = box.value?.clientWidth || 0 }); ro.observe(box.value) }
})
onBeforeUnmount(() => ro?.disconnect())
const COL = computed(() => {
  const n = Math.max(1, props.flow.length)
  return Math.min(MAXCOL, Math.max(MINCOL, Math.floor((avail.value - PADL * 2) / n)))
})

const geo = computed(() => {
  const COL_ = COL.value
  const f = props.flow
  const top = Math.max(1, ...f.map((r) => UP.reduce((s, k) => s + Math.max(0, r[k]), 0)), ...f.map((r) => r.cum))
  const bot = Math.max(1, ...f.map((r) => DOWN.reduce((s, k) => s + Math.abs(Math.min(0, r[k])), 0)), ...f.map((r) => -r.cum))
  const k = (H - PADT - PADB) / (top + bot)
  const zero = PADT + top * k
  const bars = []
  f.forEach((r, i) => {
    const x = PADL + i * COL_ + 5
    let up = zero, dn = zero
    for (const key of UP) { const v = Math.max(0, r[key]); if (v) { up -= v * k; bars.push({ key, x, y: up, h: v * k, i }) } }
    for (const key of DOWN) { const v = Math.abs(Math.min(0, r[key])); if (v) { bars.push({ key, x, y: dn, h: v * k, i }); dn += v * k } }
  })
  const line = f.map((r, i) => `${PADL + i * COL_ + COL_ / 2},${zero - r.cum * k}`).join(' ')
  return { bars, zero, line, width: PADL * 2 + f.length * COL_ }
})
const dm = (iso) => `${iso.slice(8, 10)}.${iso.slice(5, 7)}`
const tip = (r) => `${dm(r.week)}\n` + BADGES.filter((b) => r[b.key]).map((b) => `${b.label}: ${formatMoney(r[b.key])}`).join('\n') + `\nИтого за неделю: ${formatMoney(r.net)}\nНакоплено: ${formatMoney(r.cum)}`
</script>

<template>
  <div class="space-y-2" data-testid="flow-chart">
    <div class="flex flex-wrap gap-3 text-[10px] uppercase tracking-wider font-bold">
      <span v-for="b in BADGES" :key="b.key" class="flex items-center gap-1 text-slate-400"><i class="w-2 h-2 rounded-sm inline-block" :style="{ background: COLORS[b.key] }" />{{ b.label }}</span>
      <span class="flex items-center gap-1 text-slate-400"><i class="w-3 h-0.5 inline-block bg-white" />Накоплено</span>
    </div>
    <div ref="box" class="overflow-x-auto">
      <svg :width="geo.width" :height="H" :viewBox="`0 0 ${geo.width} ${H}`" class="block">
        <line :x1="0" :x2="geo.width" :y1="geo.zero" :y2="geo.zero" stroke="#334155" stroke-width="1" />
        <rect v-for="(b, n) in geo.bars" :key="n" :x="b.x" :y="b.y" :width="COL - 10" :height="Math.max(1, b.h)" :fill="COLORS[b.key]" rx="1" />
        <polyline v-if="flow.length > 1" :points="geo.line" fill="none" stroke="#fff" stroke-width="1.5" stroke-linejoin="round" />
        <g v-for="(r, i) in flow" :key="r.week" data-testid="flow-week">
          <rect :x="PADL + i * COL" :y="0" :width="COL" :height="H" fill="transparent"><title>{{ tip(r) }}</title></rect>
          <text :x="PADL + i * COL + COL / 2" :y="H - 6" text-anchor="middle" font-size="9" fill="#64748b">{{ dm(r.week) }}</text>
        </g>
      </svg>
    </div>
  </div>
</template>

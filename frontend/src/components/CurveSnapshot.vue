<script setup>
import { dmy, numText } from '../utils/numfmt.js'
import { computed } from 'vue'
import { curveSnapshot, curveChart, lastPoint, plainPct } from '../utils/macro.js'
import UiCard from './ui/UiCard.vue'

// Снимок кривой доходности Treasury: как выглядит кривая сегодня и как выглядела год и пять лет назад.
// Сроки идут по горизонтали равномерно (так кривую и принято рисовать), значение — по вертикали.
const props = defineProps({
  series: { type: Object, default: () => ({}) }, // ust_1m … ust_30y (10 лет — ust_10y из источника «us»)
  today: { type: String, required: true }
})

const WHEN = [
  { key: 'now', years: 0, label: 'сейчас', color: '#38bdf8' },
  { key: 'y1', years: 1, label: 'год назад', color: '#a78bfa' },
  { key: 'y5', years: 5, label: '5 лет назад', color: '#64748b' }
]
const snaps = computed(() => WHEN.map((w) => ({ ...w, points: curveSnapshot(props.series, props.today, w.years) })))
const chart = computed(() => curveChart(snaps.value))
// Дата последней точки: от неё «сейчас» (сегодня данные могут быть ещё не опубликованы)
const asOf = computed(() => lastPoint(props.series.ust_10y)?.d || '')
const tenYears = (s) => s.points.find((p) => p.id === 'ust_10y')?.v ?? null
</script>

<template>
  <UiCard class="p-3 space-y-2" data-testid="curve-snapshot">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      <h3 class="font-bold text-slate-300 uppercase tracking-wider">Кривая доходности Treasury</h3>
      <span v-for="s in snaps" :key="s.key" class="inline-flex items-center gap-1.5 text-slate-400">
        <i class="inline-block w-3 h-0.5 rounded" :style="{ backgroundColor: s.color }"></i>{{ s.label }}
        <b v-if="tenYears(s) !== null" class="tabular-nums text-slate-300" title="10 лет">{{ plainPct(tenYears(s)) }}</b>
      </span>
      <span v-if="asOf" class="ml-auto text-slate-600">последняя точка {{ dmy(asOf) }}</span>
    </div>
    <p v-if="chart.empty" class="text-xs text-slate-500">Нет данных. Нажми «Обновить»: подтянется кривая доходности Treasury из FRED</p>
    <svg v-else :viewBox="`0 0 ${chart.width} ${chart.height}`" class="w-full h-auto" role="img" aria-label="Кривая доходности Treasury">
      <g v-for="t in chart.yTicks" :key="t.v">
        <line :x1="chart.pad.l" :x2="chart.width - chart.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-width="1" />
        <text :x="chart.pad.l - 6" :y="t.y + 3" text-anchor="end" font-size="10" fill="#64748b">{{ numText(t.v, 3) }} %</text>
      </g>
      <text v-for="x in chart.xs" :key="x.label" :x="x.x" :y="chart.height - 14" text-anchor="middle" font-size="10" fill="#64748b">{{ x.label }}</text>
      <path v-for="l in [...chart.lines].reverse()" :key="l.key" :d="l.d" fill="none" :stroke="l.color" :stroke-width="l.key === 'now' ? 2.2 : 1.4" stroke-linejoin="round" :data-line="l.key" />
      <!-- срок без данных между двумя известными: пунктир -->
      <path v-for="l in [...chart.lines].reverse().filter((x) => x.dg)" :key="'gap-' + l.key" :d="l.dg" fill="none" :stroke="l.color" :stroke-width="l.key === 'now' ? 2.2 : 1.4" stroke-dasharray="4 4" :data-gap="l.key" />
    </svg>
    <p class="text-[11px] text-slate-600">Нормальная кривая идёт вверх: чем длиннее срок, тем выше доходность. Когда короткие ставки выше длинных, кривую называют инвертированной.</p>
  </UiCard>
</template>

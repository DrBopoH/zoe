<script setup>
import { computed } from 'vue'
import { buildGrowthChart } from '../utils/growthChart.js'
import { moneyText } from '../utils/marketTable.js'

// Два графика роста вложения: «сравнимый» (все бумаги на одном горизонте) и «реальный» (покупка сегодня, линия кончается в день погашения).
// items: [{ id, name, color, top, sc }] (sc из scoreBonds). Лучшие (top) — толстые линии с точками, остальные — тонкие фоном.
const props = defineProps({
  items: { type: Array, default: () => [] },
  amount: { type: Number, default: 10000 },
  currency: { type: String, default: 'UAH' }
})

const rel = computed(() => buildGrowthChart({ items: props.items, mode: 'relative' }))
const abs = computed(() => buildGrowthChart({ items: props.items, mode: 'absolute', amount: props.amount }))
const tops = computed(() => props.items.filter((i) => i.top))
const order = (c) => [...c.lines].sort((a, b) => Number(a.top) - Number(b.top)) // лучшие рисуем поверх
const byId = computed(() => new Map(props.items.map((i) => [i.id, i])))
const xLabel = (c) => ({ 'text-anchor': 'middle', y: c.height - 6 })
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-3">
    <div v-for="(chart, idx) in [{ c: rel, title: 'Сравнимо: рост за 10 лет, во сколько раз', rel: true }, { c: abs, title: 'Реально: покупка сегодня, линия кончается в день погашения', rel: false }]" :key="idx" class="space-y-1">
      <div class="text-[10px] text-slate-500 uppercase tracking-wider font-bold">{{ chart.title }}</div>
      <svg :viewBox="`0 0 ${chart.c.width} ${chart.c.height}`" class="w-full h-auto" role="img">
        <defs>
          <clipPath :id="`plot-${idx}`"><rect :x="chart.c.pad.l" :y="chart.c.pad.t" :width="chart.c.width - chart.c.pad.l - chart.c.pad.r" :height="chart.c.height - chart.c.pad.t - chart.c.pad.b" /></clipPath>
        </defs>
        <g class="text-[10px]" fill="#64748b">
          <g v-for="t in chart.c.yTicks" :key="'y' + t.v">
            <line :x1="chart.c.pad.l" :x2="chart.c.width - chart.c.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-dasharray="3 3" />
            <text :x="chart.c.pad.l - 6" :y="t.y + 3" text-anchor="end">{{ chart.rel ? `×${t.v}` : moneyText(t.v, currency) }}</text>
          </g>
          <text v-for="t in chart.c.xTicks" :key="'x' + t.label" :x="t.x" v-bind="xLabel(chart.c)">{{ t.label }}</text>
        </g>
        <g :clip-path="`url(#plot-${idx})`">
          <template v-for="l in order(chart.c)" :key="l.id">
            <path :d="l.solid" fill="none" :stroke="l.color" :stroke-width="l.top ? 2.4 : 1" :stroke-opacity="l.top ? 1 : 0.35" />
            <path v-if="l.dashed" :d="l.dashed" fill="none" :stroke="l.color" :stroke-width="l.top ? 1.6 : 0.8" :stroke-opacity="l.top ? 0.8 : 0.25" stroke-dasharray="4 4" />
          </template>
        </g>
        <template v-for="l in chart.c.lines.filter((x) => x.top)" :key="'d' + l.id">
          <g v-if="l.mat">
            <circle :cx="l.mat.x" :cy="l.mat.y" r="7" :fill="l.color" stroke="#020617" stroke-width="1.5">
              <title>{{ byId.get(l.id)?.name }} · {{ l.id }}: погашение, {{ chart.rel ? '×' + (Math.round(byId.get(l.id).sc.multipleT * 100) / 100) : moneyText(l.end.v, currency) }}</title>
            </circle>
            <text :x="l.mat.x" :y="l.mat.y + 3.5" text-anchor="middle" font-size="10" font-weight="700" fill="#020617" pointer-events="none">{{ byId.get(l.id)?.place }}</text>
          </g>
          <circle v-if="chart.rel && l.end" :cx="l.end.x" :cy="l.end.y" r="3" :fill="l.color" fill-opacity="0.6" />
        </template>
      </svg>
    </div>

    <div v-if="tops.length" class="flex flex-wrap gap-x-5 gap-y-1 text-[11px] text-slate-400">
      <span v-for="i in tops" :key="i.id" class="inline-flex items-center gap-1.5">
        <span class="inline-flex items-center justify-center w-4 h-4 rounded-full text-[9px] font-bold text-slate-950" :style="{ backgroundColor: i.color }">{{ i.place }}</span>
        <span class="text-slate-300">{{ i.id }}</span>
        <span>{{ i.name }}</span>
        <span class="text-slate-500">→ {{ moneyText(amount * i.sc.multipleT, currency) }} к погашению, {{ moneyText(amount * i.sc.multipleH, currency) }} за 10 л.</span>
      </span>
    </div>
    <div v-else class="text-[11px] text-slate-600 uppercase tracking-wider">Нет бумаг с оценкой для графика</div>
  </section>
</template>

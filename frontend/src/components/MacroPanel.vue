<script setup>
import { ref, computed } from 'vue'
import { SERIES, RANGES, buildMacroChart, macroSummary, signedPct, plainPct } from '../utils/macro.js'

// Ставка НБУ и инфляция: текущие значения и график. Данные — из кеша (/api/macro), заполняются кнопкой «Обновить».
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true }
})

const WIDTH = 1000
const HEIGHT = 220
const range = ref('10')
const IDS = ['policy_rate', 'cpi_yoy', 'core_yoy']

const has = computed(() => IDS.some((id) => props.series[id]?.length))
const sum = computed(() => macroSummary(props.series, props.today))
const chart = computed(() => buildMacroChart({
  series: props.series,
  ids: IDS,
  years: RANGES.find((r) => r.key === range.value)?.years ?? 10,
  todayIso: props.today,
  width: WIDTH,
  height: HEIGHT
}))
const monthText = (ym) => (ym ? `${ym.slice(5, 7)}.${ym.slice(0, 4)}` : '')
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3 text-[10px] uppercase tracking-wider font-bold">
      <span class="text-slate-500">Ставка НБУ и инфляция</span>
      <div v-if="has" class="flex items-center gap-1">
        <button
          v-for="r in RANGES"
          :key="r.key"
          type="button"
          class="px-2 py-1 rounded border cursor-pointer transition-colors"
          :class="r.key === range ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'text-slate-500 border-transparent hover:text-slate-300'"
          @click="range = r.key"
        >{{ r.label }}</button>
      </div>
    </div>

    <div v-if="!has" class="text-xs text-slate-600 uppercase tracking-wider py-6 text-center">Нет данных. Нажми «Обновить»: подтянутся ставки и инфляция НБУ</div>
    <template v-else>
      <div class="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-xs">
        <span class="text-slate-400">Облікова ставка <b class="text-emerald-400 tabular-nums">{{ plainPct(sum.policy) }}</b></span>
        <span class="text-slate-400" :title="`Последний известный месяц: ${monthText(sum.cpiMonth)}`">Инфляция г/г <b class="text-amber-300 tabular-nums">{{ plainPct(sum.cpi) }}</b><span class="text-slate-600"> · {{ monthText(sum.cpiMonth) }}</span></span>
        <span class="text-slate-400">Базовая <b class="text-orange-300 tabular-nums">{{ plainPct(sum.core) }}</b></span>
        <span v-if="sum.overnightCD !== null" class="text-slate-400" title="Ставка по депозитным сертификатам овернайт">Сертификаты овернайт <b class="text-sky-300 tabular-nums">{{ plainPct(sum.overnightCD) }}</b></span>
        <span v-if="sum.realPolicy !== null" class="text-slate-400" title="Облікова ставка с поправкой на инфляцию: (1 + ставка) / (1 + инфляция) − 1">Реальная ставка <b class="tabular-nums" :class="sum.realPolicy >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(sum.realPolicy) }}</b></span>
      </div>

      <svg :viewBox="`0 0 ${WIDTH} ${HEIGHT}`" class="w-full h-auto select-none" role="img" aria-label="Ставка НБУ и инфляция">
        <g class="text-[10px]" fill="#64748b">
          <g v-for="t in chart.yTicks" :key="'y' + t.v">
            <line :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-dasharray="3 3" />
            <text :x="chart.pad.l - 6" :y="t.y + 3" text-anchor="end">{{ t.v }} %</text>
          </g>
          <text v-for="t in chart.xTicks" :key="'x' + t.label" :x="t.x" :y="HEIGHT - 8" text-anchor="middle">{{ t.label }}</text>
        </g>
        <line v-if="chart.zeroY !== null" :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="chart.zeroY" :y2="chart.zeroY" stroke="#475569" />
        <path v-for="l in chart.lines" :key="l.id" :d="l.d" fill="none" :stroke="l.color" :stroke-width="l.id === 'core_yoy' ? 1.2 : 2" :stroke-opacity="l.id === 'core_yoy' ? 0.7 : 1" stroke-linejoin="round" />
      </svg>

      <div class="flex flex-wrap items-center gap-x-5 gap-y-1 text-[11px] text-slate-400">
        <span v-for="l in chart.lines" :key="l.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: SERIES[l.id].color }"></span>{{ l.label }}</span>
        <span class="text-slate-600">Колонка «Реальная» в таблице: доходность бумаги с поправкой на последнюю инфляцию г/г (только гривна)</span>
      </div>
    </template>
  </section>
</template>

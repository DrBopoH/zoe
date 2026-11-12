<script setup>
import { ref, computed } from 'vue'
import { SERIES, RANGES, MACRO_PAD, MACRO_MIN_SPAN, MACRO_MAX_SPAN, buildMacroChart, macroSummary, macroWindow, macroRanges, isoAt, valueAt, signedPct, plainPct } from '../utils/macro.js'
import { useChartView } from '../composables/useChartView.js'

// Ставка НБУ и инфляция: текущие значения и график. Данные — из кеша (/api/macro), заполняются кнопкой «Обновить».
// Интерактив (колесо, перетаскивание, наведение, сброс) — общий движок графиков, как у графика роста вложения.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true }
})

const WIDTH = 1000
const HEIGHT = 220
const IDS = ['policy_rate', 'cpi_yoy', 'core_yoy']
const rangeKey = ref('10') // кнопка 5 л. / 10 л. / Всё: задаёт окно, пока его не тронули колесом или мышью

const windowFor = (key) => macroWindow({ series: props.series, ids: IDS, todayIso: props.today, years: RANGES.find((r) => r.key === key)?.years ?? 10 })
const kv = useChartView({
  width: WIDTH,
  height: HEIGHT,
  pad: MACRO_PAD,
  autoX: () => windowFor(rangeKey.value),
  ranges: (view) => macroRanges({ series: props.series, ids: IDS, view, todayIso: props.today }),
  spans: { minSpan: MACRO_MIN_SPAN, maxSpan: MACRO_MAX_SPAN },
  resetOn: () => [props.series]
})
const { svgRef, onWheel, onDown, onMove, onUp, onLeave, reset, zoomed } = kv
const pick = (key) => { rangeKey.value = key; kv.reset() } // новое окно по умолчанию, ручная вертикаль сбрасывается

const has = computed(() => IDS.some((id) => props.series[id]?.length))
const sum = computed(() => macroSummary(props.series, props.today))
const chart = computed(() => buildMacroChart({ series: props.series, ids: IDS, view: kv.view.value, yRange: kv.yShown.value || kv.target.value, todayIso: props.today, width: WIDTH, height: HEIGHT }))
const monthText = (ym) => (ym ? `${ym.slice(5, 7)}.${ym.slice(0, 4)}` : '')

// Подсказка под курсором: дата и значения рядов на неё
const hover = computed(() => {
  if (kv.hoverX.value === null || kv.hoverX.value > 0.05) return null
  const iso = isoAt(kv.hoverX.value, props.today)
  const rows = IDS.map((id) => ({ id, label: SERIES[id].label, color: SERIES[id].color, v: valueAt(props.series[id], iso) })).filter((r) => r.v !== null)
  return rows.length ? { date: iso.split('-').reverse().join('.'), rows } : null
})
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
          :class="r.key === rangeKey && !zoomed ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'text-slate-500 border-transparent hover:text-slate-300'"
          @click="pick(r.key)"
        >{{ r.label }}</button>
        <button type="button" class="ml-2 text-slate-400 hover:text-emerald-400 underline whitespace-nowrap cursor-pointer" :class="{ 'opacity-40 pointer-events-none': !zoomed }" @click="reset">Сброс масштаба</button>
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

      <svg
        ref="svgRef"
        :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
        class="w-full h-auto select-none touch-none"
        :class="zoomed ? 'cursor-grab active:cursor-grabbing' : 'cursor-crosshair'"
        role="img"
        aria-label="Ставка НБУ и инфляция"
        @wheel.prevent="onWheel"
        @pointerdown="onDown"
        @pointermove="onMove"
        @pointerup="onUp"
        @pointerleave="onLeave"
        @dblclick="reset"
      >
        <defs>
          <clipPath id="macro-clip"><rect :x="chart.pad.l" :y="chart.pad.t" :width="WIDTH - chart.pad.l - chart.pad.r" :height="HEIGHT - chart.pad.t - chart.pad.b" /></clipPath>
        </defs>
        <g class="text-[10px]" fill="#64748b">
          <g v-for="t in chart.yTicks" :key="'y' + t.v">
            <line :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-dasharray="3 3" />
            <text :x="chart.pad.l - 6" :y="t.y + 3" text-anchor="end">{{ t.v }} %</text>
          </g>
          <text v-for="t in chart.xTicks" :key="'x' + t.label" :x="t.x" :y="HEIGHT - 8" text-anchor="middle">{{ t.label }}</text>
        </g>
        <g clip-path="url(#macro-clip)">
        <line v-if="chart.zeroY !== null" :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="chart.zeroY" :y2="chart.zeroY" stroke="#475569" />
        <line v-if="kv.hoverPx.value !== null && kv.hoverX.value <= 0.05" :x1="kv.hoverPx.value" :x2="kv.hoverPx.value" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#64748b" stroke-opacity="0.6" />
        <path v-for="l in chart.lines" :key="l.id" :d="l.d" fill="none" :stroke="l.color" :stroke-width="l.id === 'core_yoy' ? 1.2 : 2" :stroke-opacity="l.id === 'core_yoy' ? 0.7 : 1" stroke-linejoin="round" />
        </g>
      </svg>

      <div class="flex flex-wrap items-center gap-x-5 gap-y-1 text-[11px] text-slate-400 min-h-[1.5rem]">
        <template v-if="hover">
          <span class="text-slate-200 font-bold">{{ hover.date }}</span>
          <span v-for="r in hover.rows" :key="r.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: r.color }"></span>{{ r.label }} <b class="tabular-nums text-slate-200">{{ plainPct(r.v) }}</b></span>
        </template>
        <template v-else>
        <span v-for="l in chart.lines" :key="l.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: SERIES[l.id].color }"></span>{{ l.label }}</span>
        <span class="text-slate-600">Колесо: масштаб, перетаскивание: сдвиг, двойной клик: сброс. Колонка «Реальная» в таблице: доходность бумаги с поправкой на последнюю инфляцию г/г (только гривна)</span>
        </template>
      </div>
    </template>
  </section>
</template>

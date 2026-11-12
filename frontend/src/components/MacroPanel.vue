<script setup>
import { ref, computed } from 'vue'
import { SERIES, RANGES, MACRO_PAD, MACRO_MIN_SPAN, MACRO_MAX_SPAN, buildMacroChart, macroWindow, macroRanges, isoAt, valueAt, unitText, axisText } from '../utils/macro.js'
import { useChartView } from '../composables/useChartView.js'

// Универсальная панель макро-графика: заголовок, кнопки окна, слот summary (текущие значения), график и подсказка под курсором. Данные — из кеша (/api/macro), заполняются кнопкой «Обновить».
// Интерактив (колесо, перетаскивание, наведение, сброс) — общий движок графиков, как у графика роста вложения.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  title: { type: String, required: true },
  ids: { type: Array, required: true }, // какие ряды рисовать
  ranges: { type: Array, default: () => RANGES }, // кнопки окна (из RANGES)
  defaultRange: { type: String, default: '10' },
  unit: { type: String, default: '%' }, // единица значения: % | ₴ | $
  empty: { type: String, default: 'Нет данных. Нажми «Обновить»: подтянутся ставки и инфляция НБУ' },
  hint: { type: String, default: 'Колесо: масштаб, перетаскивание: сдвиг, двойной клик: сброс' }
})

let uid = 0
const clipId = `macro-clip-${++uid}-${Math.random().toString(36).slice(2, 7)}`
const WIDTH = 1000
const HEIGHT = 220
const IDS = computed(() => props.ids)
const rangeKey = ref(props.defaultRange) // кнопки окна задают его, пока его не тронули колесом или мышью

const windowFor = (key) => {
  const r = RANGES.find((x) => x.key === key)
  return macroWindow({ series: props.series, ids: IDS.value, todayIso: props.today, years: r?.years ?? 10, from: r?.from || null })
}
const kv = useChartView({
  width: WIDTH,
  height: HEIGHT,
  pad: MACRO_PAD,
  autoX: () => windowFor(rangeKey.value),
  ranges: (view) => macroRanges({ series: props.series, ids: IDS.value, view, todayIso: props.today, floor: props.unit === '%' ? 0 : null }),
  spans: { minSpan: MACRO_MIN_SPAN, maxSpan: MACRO_MAX_SPAN },
  resetOn: () => [props.series, props.ids]
})
const { svgRef, onWheel, onDown, onMove, onUp, onLeave, reset, zoomed } = kv
const pick = (key) => { rangeKey.value = key; kv.reset() } // новое окно по умолчанию, ручная вертикаль сбрасывается

const has = computed(() => IDS.value.some((id) => props.series[id]?.length))
const chart = computed(() => buildMacroChart({ series: props.series, ids: IDS.value, view: kv.view.value, yRange: kv.yShown.value || kv.target.value, todayIso: props.today, width: WIDTH, height: HEIGHT }))

// Подсказка под курсором: дата и значения рядов на неё
const hover = computed(() => {
  if (kv.hoverX.value === null || kv.hoverX.value > 0.05) return null
  const iso = isoAt(kv.hoverX.value, props.today)
  const rows = IDS.value.map((id) => ({ id, label: SERIES[id].label, color: SERIES[id].color, v: valueAt(props.series[id], iso) })).filter((r) => r.v !== null)
  return rows.length ? { date: iso.split('-').reverse().join('.'), rows } : null
})
</script>


<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3 text-[10px] uppercase tracking-wider font-bold">
      <span class="text-slate-500">{{ title }}</span>
      <div v-if="has" class="flex items-center gap-1">
        <button
          v-for="r in ranges"
          :key="r.key"
          type="button"
          class="px-2 py-1 rounded border cursor-pointer transition-colors"
          :class="r.key === rangeKey && !zoomed ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'text-slate-500 border-transparent hover:text-slate-300'"
          @click="pick(r.key)"
        >{{ r.label }}</button>
        <button type="button" class="ml-2 text-slate-400 hover:text-emerald-400 underline whitespace-nowrap cursor-pointer" :class="{ 'opacity-40 pointer-events-none': !zoomed }" @click="reset">Сброс масштаба</button>
      </div>
    </div>

    <div v-if="!has" class="text-xs text-slate-600 uppercase tracking-wider py-6 text-center">{{ empty }}</div>
    <template v-else>
      <div class="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-xs"><slot name="summary" /></div>

      <svg
        ref="svgRef"
        :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
        class="w-full h-auto select-none touch-none"
        :class="zoomed ? 'cursor-grab active:cursor-grabbing' : 'cursor-crosshair'"
        role="img"
        :aria-label="title"
        @wheel.prevent="onWheel"
        @pointerdown="onDown"
        @pointermove="onMove"
        @pointerup="onUp"
        @pointerleave="onLeave"
        @dblclick="reset"
      >
        <defs>
          <clipPath :id="clipId"><rect :x="chart.pad.l" :y="chart.pad.t" :width="WIDTH - chart.pad.l - chart.pad.r" :height="HEIGHT - chart.pad.t - chart.pad.b" /></clipPath>
        </defs>
        <g class="text-[10px]" fill="#64748b">
          <g v-for="t in chart.yTicks" :key="'y' + t.v">
            <line :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-dasharray="3 3" />
            <text :x="chart.pad.l - 6" :y="t.y + 3" text-anchor="end">{{ axisText(t.v, unit) }}</text>
          </g>
          <text v-for="t in chart.xTicks" :key="'x' + t.label" :x="t.x" :y="HEIGHT - 8" text-anchor="middle">{{ t.label }}</text>
        </g>
        <g :clip-path="`url(#${clipId})`">
        <line v-if="chart.zeroY !== null" :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="chart.zeroY" :y2="chart.zeroY" stroke="#475569" />
        <line v-if="kv.hoverPx.value !== null && kv.hoverX.value <= 0.05" :x1="kv.hoverPx.value" :x2="kv.hoverPx.value" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#64748b" stroke-opacity="0.6" />
        <path v-for="l in chart.lines" :key="l.id" :d="l.d" fill="none" :stroke="l.color" :stroke-width="SERIES[l.id].thin ? 1.2 : 2" :stroke-opacity="SERIES[l.id].thin ? 0.7 : 1" stroke-linejoin="round" />
        </g>
      </svg>

      <div class="flex flex-wrap items-center gap-x-5 gap-y-1 text-[11px] text-slate-400 min-h-[1.5rem]">
        <template v-if="hover">
          <span class="text-slate-200 font-bold">{{ hover.date }}</span>
          <span v-for="r in hover.rows" :key="r.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: r.color }"></span>{{ r.label }} <b class="tabular-nums text-slate-200">{{ unitText(r.v, unit) }}</b></span>
        </template>
        <template v-else>
        <span v-for="l in chart.lines" :key="l.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: SERIES[l.id].color }"></span>{{ l.label }}</span>
        <span class="text-slate-600">{{ hint }}</span>
        </template>
      </div>
    </template>
  </section>
</template>

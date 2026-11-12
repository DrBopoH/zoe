<script setup>
import { computed } from 'vue'
import { SERIES, MACRO_PAD, MACRO_MIN_SPAN, MACRO_MAX_SPAN, buildMacroChart, macroWindow, macroRanges, isoAt, valueAt, unitText, axisText } from '../utils/macro.js'
import { BENNER, bennerPhase } from '../utils/benner.js'
import BennerBands from './BennerBands.vue'
import { useChartView } from '../composables/useChartView.js'

// Универсальная панель макро-графика: заголовок, кнопки окна, слот summary (текущие значения), график и подсказка под курсором. Данные — из кеша (/api/macro), заполняются кнопкой «Обновить».
// Интерактив (колесо, перетаскивание, наведение, сброс) — общий движок графиков, как у графика роста вложения.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  title: { type: String, required: true },
  ids: { type: Array, required: true }, // какие ряды рисовать
  unit: { type: String, default: '%' }, // единица значения: % | ₴ | $
  empty: { type: String, default: 'Нет данных. Нажми «Обновить»: подтянутся ставки и инфляция НБУ' },
  benner: { type: Boolean, default: true }, // полоса лет цикла Беннера внизу графика
  hint: { type: String, default: 'Колесо: масштаб, перетаскивание: сдвиг, двойной клик: сброс' }
})

let uid = 0
const clipId = `macro-clip-${++uid}-${Math.random().toString(36).slice(2, 7)}`
const WIDTH = 1000
const HEIGHT = 220
const IDS = computed(() => props.ids)

// Окно по умолчанию — весь период данных; дальше колесо и перетаскивание
const windowAll = () => macroWindow({ series: props.series, ids: IDS.value, todayIso: props.today, years: 0 })
const kv = useChartView({
  width: WIDTH,
  height: HEIGHT,
  pad: MACRO_PAD,
  autoX: windowAll,
  ranges: (view) => macroRanges({ series: props.series, ids: IDS.value, view, todayIso: props.today, floor: props.unit === '%' ? 0 : null }),
  spans: { minSpan: MACRO_MIN_SPAN, maxSpan: MACRO_MAX_SPAN },
  resetOn: () => [props.series, props.ids]
})
const { svgRef, onWheel, onDown, onMove, onUp, onLeave, reset, zoomed } = kv

const has = computed(() => IDS.value.some((id) => props.series[id]?.length))
const chart = computed(() => buildMacroChart({ series: props.series, ids: IDS.value, view: kv.view.value, yRange: kv.yShown.value || kv.target.value, todayIso: props.today, width: WIDTH, height: HEIGHT }))

// Слой цикла Беннера (components/BennerBands.vue): ось графика — годы от сегодня
const YEAR_MS = 365.25 * 86400000
const todayMs = computed(() => Date.parse(`${props.today}T00:00:00Z`))
const bn = computed(() => {
  const v = kv.view.value, w = WIDTH - MACRO_PAD.l - MACRO_PAD.r
  return { fromMs: todayMs.value + v.x0 * YEAR_MS, toMs: todayMs.value + v.x1 * YEAR_MS, xOf: (ms) => MACRO_PAD.l + (((ms - todayMs.value) / YEAR_MS - v.x0) / (v.x1 - v.x0)) * w }
})
const bennerNow = computed(() => {
  if (!props.benner || kv.hoverX.value === null || kv.hoverX.value > 0.05) return null
  const y = Number(isoAt(kv.hoverX.value, props.today).slice(0, 4))
  const ph = bennerPhase(y)
  return ph ? { year: y, ...BENNER[ph.kind], kind: ph.kind, key: ph.key } : null
})

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
      <button v-if="has && zoomed" type="button" class="text-slate-400 hover:text-emerald-400 underline whitespace-nowrap cursor-pointer" @click="reset">Сброс масштаба</button>
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
        <BennerBands v-if="benner" layer="bg" v-bind="bn" :left="MACRO_PAD.l" :right="WIDTH - MACRO_PAD.r" :top="MACRO_PAD.t" :bottom="HEIGHT - MACRO_PAD.b" />
        <g :clip-path="`url(#${clipId})`">
        <line v-if="chart.zeroY !== null" :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="chart.zeroY" :y2="chart.zeroY" stroke="#475569" />
        <line v-if="kv.hoverPx.value !== null && kv.hoverX.value <= 0.05" :x1="kv.hoverPx.value" :x2="kv.hoverPx.value" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#64748b" stroke-opacity="0.6" />
        <path v-for="l in chart.lines" :key="l.id" :d="l.d" fill="none" :stroke="l.color" :stroke-width="SERIES[l.id].thin ? 1.2 : 2" :stroke-opacity="SERIES[l.id].thin ? 0.7 : 1" stroke-linejoin="round" />
        </g>
        <BennerBands v-if="benner" layer="strip" v-bind="bn" :left="MACRO_PAD.l" :right="WIDTH - MACRO_PAD.r" :strip-y="HEIGHT - MACRO_PAD.b + 5" />
      </svg>

      <div class="flex flex-wrap items-center gap-x-5 gap-y-1 text-[11px] text-slate-400 min-h-[1.5rem]">
        <template v-if="hover">
          <span class="text-slate-200 font-bold">{{ hover.date }}</span>
          <span v-for="r in hover.rows" :key="r.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: r.color }"></span>{{ r.label }} <b class="tabular-nums text-slate-200">{{ unitText(r.v, unit) }}</b></span>
          <span v-if="bennerNow" class="inline-flex items-center gap-1.5" title="Цикл Беннера — курьёз, не сигнал"><span class="inline-block w-2 h-2 rounded-sm" :style="{ backgroundColor: bennerNow.color }"></span>Беннер {{ bennerNow.year }}: <b class="text-slate-200">{{ bennerNow.key ? bennerNow.kind + ' · ' + bennerNow.label : 'после: ' + bennerNow.label }}</b></span>
        </template>
        <template v-else>
        <span v-for="l in chart.lines" :key="l.id" class="inline-flex items-center gap-1.5"><span class="inline-block w-3 h-0.5" :style="{ backgroundColor: SERIES[l.id].color }"></span>{{ l.label }}</span>
        <span class="text-slate-600">{{ hint }}</span>
        <span v-if="benner" class="text-slate-600" title="Цикл Беннера (1875). Исторический курьёз, предсказательной силы не доказано">цикл Беннера: A паника · B пик · C дно, плотный — сам год, бледный — период после (курьёз, не сигнал)</span>
        </template>
      </div>
    </template>
  </section>
</template>

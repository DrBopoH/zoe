<script setup>
import { computed, ref, watch } from 'vue'
import { buildGrowthChart, autoView, zoomView, panView } from '../utils/growthChart.js'
import { moneyText } from '../utils/marketTable.js'

// Один график роста вложения: годы от сегодня по горизонтали (прошлое слева), сумма по вертикали. Линия на бумагу, цвет = цвет
// рамки бумаги. Лучшие (top) — толстые с номерами в точке погашения, остальные тонко фоном; сгоревшие (уже погашенные) — серым
// пунктиром от выпуска до погашения. Колесо — масштаб по времени (якорь под курсором), перетаскивание — сдвиг, двойной клик — сброс.
// items: [{ id, name, color, top, place, sc }], matured: [{ id, rate, startT, endT }]
const props = defineProps({
  items: { type: Array, default: () => [] },
  matured: { type: Array, default: () => [] },
  amount: { type: Number, default: 10000 },
  currency: { type: String, default: 'UAH' },
  today: { type: String, required: true }
})

const WIDTH = 1000, HEIGHT = 300
const view = ref(null) // null — окно по умолчанию (следует за данными), иначе { x0, x1 } после масштаба или сдвига
const auto = computed(() => autoView(props.items, props.matured))
const cur = computed(() => view.value || auto.value)
const chart = computed(() => buildGrowthChart({ items: props.items, matured: props.matured, amount: props.amount, view: cur.value, today: props.today, width: WIDTH, height: HEIGHT }))
watch(() => [props.items, props.matured], () => { view.value = null }) // другие фильтры — снова окно по умолчанию

const svgRef = ref(null)
// Координата мыши → единицы viewBox (график масштабируется по ширине страницы)
const px = (e) => {
  const r = svgRef.value.getBoundingClientRect()
  return ((e.clientX - r.left) / r.width) * WIDTH
}
const onWheel = (e) => {
  const anchor = chart.value.tAt(px(e))
  view.value = zoomView(cur.value, e.deltaY < 0 ? 1 / 1.2 : 1.2, anchor)
}
let drag = null
const onDown = (e) => { drag = { x: px(e), view: cur.value }; svgRef.value.setPointerCapture(e.pointerId) }
const hoverT = ref(null)
const onMove = (e) => {
  const x = px(e)
  hoverT.value = chart.value.tAt(x)
  if (!drag) return
  const dt = ((drag.x - x) / (WIDTH - chart.value.pad.l - chart.value.pad.r)) * (drag.view.x1 - drag.view.x0)
  view.value = panView(drag.view, dt)
}
const onUp = () => { drag = null }
const onLeave = () => { hoverT.value = null; drag = null }
const reset = () => { view.value = null }

const tops = computed(() => props.items.filter((i) => i.top))
const byId = computed(() => new Map(props.items.map((i) => [i.id, i])))
const order = computed(() => [...chart.value.lines].sort((a, b) => Number(a.top) - Number(b.top))) // лучшие рисуем поверх

// Подсказка под курсором: дата и суммы лучших пяти на этот момент
const dateAt = (t) => new Date(Date.parse(`${props.today}T00:00:00Z`) + t * 365.25 * 86400000).toLocaleDateString('ru-RU', { timeZone: 'UTC' })
const hoverX = computed(() => (hoverT.value === null ? null : chart.value.pad.l + ((hoverT.value - cur.value.x0) / (cur.value.x1 - cur.value.x0)) * (WIDTH - chart.value.pad.l - chart.value.pad.r)))
const hoverInfo = computed(() => {
  if (hoverT.value === null || hoverT.value < 0) return null
  return { date: dateAt(hoverT.value), rows: tops.value.filter((i) => hoverT.value <= i.sc.years).map((i) => ({ place: i.place, color: i.color, v: chart.value.valueOf(i.id, hoverT.value) })) }
})
const zoomed = computed(() => view.value !== null)
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-3">
    <div class="flex items-center justify-between gap-3 text-[10px] uppercase tracking-wider font-bold">
      <span class="text-slate-500">Рост вложения: покупка сегодня, линия кончается в день погашения; пунктир после погашения — деньги под базовой ставкой</span>
      <button type="button" class="text-slate-400 hover:text-emerald-400 underline whitespace-nowrap cursor-pointer" :class="{ 'opacity-40 pointer-events-none': !zoomed }" @click="reset">Сброс масштаба</button>
    </div>

    <svg
      ref="svgRef"
      :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
      class="w-full h-auto select-none touch-none"
      :class="zoomed ? 'cursor-grab active:cursor-grabbing' : 'cursor-crosshair'"
      role="img"
      @wheel.prevent="onWheel"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointerleave="onLeave"
      @dblclick="reset"
    >
      <defs>
        <clipPath id="plot-clip"><rect :x="chart.pad.l" :y="chart.pad.t" :width="WIDTH - chart.pad.l - chart.pad.r" :height="HEIGHT - chart.pad.t - chart.pad.b" /></clipPath>
      </defs>
      <g class="text-[10px]" fill="#64748b">
        <g v-for="t in chart.yTicks" :key="'y' + t.v">
          <line :x1="chart.pad.l" :x2="WIDTH - chart.pad.r" :y1="t.y" :y2="t.y" stroke="#1e293b" stroke-dasharray="3 3" />
          <text :x="chart.pad.l - 6" :y="t.y + 3" text-anchor="end">{{ moneyText(t.v, currency) }}</text>
        </g>
        <text v-for="t in chart.xTicks" :key="'x' + t.label" :x="t.x" :y="HEIGHT - 8" text-anchor="middle">{{ t.label }}</text>
      </g>
      <g :clip-path="`url(#plot-clip)`">
        <line :x1="chart.nowX" :x2="chart.nowX" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#475569" stroke-dasharray="2 3" />
        <line :x1="chart.horizonX" :x2="chart.horizonX" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#334155" stroke-dasharray="6 4" />
        <text :x="chart.horizonX + 4" :y="chart.pad.t + 10" font-size="9" fill="#64748b">10 л.</text>
        <text :x="chart.nowX + 4" :y="chart.pad.t + 10" font-size="9" fill="#94a3b8">сегодня</text>
        <path v-for="b in chart.burned" :key="'o' + b.id" :d="b.d" fill="none" stroke="#94a3b8" stroke-opacity="0.55" stroke-width="1.2" stroke-dasharray="4 3" />
        <template v-for="l in order" :key="l.id">
          <path :d="l.solid" fill="none" :stroke="l.color" :stroke-width="l.top ? 2.4 : 1" :stroke-opacity="l.top ? 1 : 0.35" />
          <path v-if="l.dashed" :d="l.dashed" fill="none" :stroke="l.color" :stroke-width="l.top ? 1.6 : 0.8" :stroke-opacity="l.top ? 0.8 : 0.25" stroke-dasharray="4 4" />
        </template>
        <line v-if="hoverX !== null" :x1="hoverX" :x2="hoverX" :y1="chart.pad.t" :y2="HEIGHT - chart.pad.b" stroke="#64748b" stroke-opacity="0.6" />
        <template v-for="l in chart.lines.filter((x) => x.top && x.mat)" :key="'d' + l.id">
          <circle :cx="l.mat.x" :cy="l.mat.y" r="7" :fill="l.color" stroke="#020617" stroke-width="1.5">
            <title>{{ byId.get(l.id)?.name }} · {{ l.id }}: погашение {{ dateAt(l.mat.t) }}, {{ moneyText(l.mat.v, currency) }}</title>
          </circle>
          <text :x="l.mat.x" :y="l.mat.y + 3.5" text-anchor="middle" font-size="10" font-weight="700" fill="#020617" pointer-events="none">{{ byId.get(l.id)?.place }}</text>
        </template>
      </g>
    </svg>

    <div class="flex flex-wrap items-center gap-x-5 gap-y-1 text-[11px] text-slate-400 min-h-[1.5rem]">
      <template v-if="hoverInfo">
        <span class="text-slate-200 font-bold">{{ hoverInfo.date }}</span>
        <span v-for="r in hoverInfo.rows" :key="r.place" class="inline-flex items-center gap-1.5">
          <span class="inline-flex items-center justify-center w-4 h-4 rounded-full text-[9px] font-bold text-slate-950" :style="{ backgroundColor: r.color }">{{ r.place }}</span>{{ moneyText(r.v, currency) }}
        </span>
      </template>
      <template v-else-if="tops.length">
        <span v-for="i in tops" :key="i.id" class="inline-flex items-center gap-1.5">
          <span class="inline-flex items-center justify-center w-4 h-4 rounded-full text-[9px] font-bold text-slate-950" :style="{ backgroundColor: i.color }">{{ i.place }}</span>
          <span class="text-slate-300">{{ i.id }}</span>
          <span>{{ i.name }}</span>
          <span class="text-slate-500">→ {{ moneyText(amount * i.sc.multipleT, currency) }} к погашению, {{ moneyText(amount * i.sc.multipleH, currency) }} за 10 л.</span>
        </span>
      </template>
      <span v-else class="text-slate-600 uppercase tracking-wider">Нет бумаг с оценкой для графика</span>
    </div>
    <div class="text-[10px] text-slate-600">Колесо — масштаб, перетаскивание — сдвиг, двойной клик — сбросить.</div>
  </section>
</template>

<script setup>
import { ref, computed, nextTick, onMounted } from 'vue'
import { toLocalDateStr, formatMoney, formatDateLong, formatShortDateName, formatMonthName } from '../utils/formatters.js'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  totalValueUAH: { type: Number, default: 0 }
})

const hoveredCandle = ref(null)
const chartContainerRef = ref(null)
const zoomLevel = ref(1.0)
const hasInitialScrolled = ref(false)

const svgHeight = 260
const paddingY = 30
const paddingX = 30

const dailyCandles = computed(() => {
  if (props.transactions.length === 0) return []

  const txByDate = {}
  let minDateObj = new Date()

  const sortedTx = [...props.transactions]
    .filter(t => t.time)
    .sort((a, b) => new Date(a.time) - new Date(b.time))

  sortedTx.forEach(t => {
    const dObj = new Date(t.time)
    if (dObj < minDateObj) minDateObj = dObj
    const dStr = toLocalDateStr(dObj)

    if (!txByDate[dStr]) txByDate[dStr] = []
    txByDate[dStr].push(t)
  })

  const allDays = []
  const curr = new Date(minDateObj)
  curr.setHours(0, 0, 0, 0)
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  while (curr <= today) {
    allDays.push(toLocalDateStr(curr))
    curr.setDate(curr.getDate() + 1)
  }

  let currentCap = props.totalValueUAH
  const result = []

  for (let i = allDays.length - 1; i >= 0; i--) {
    const dateStr = allDays[i]
    const dayTxs = txByDate[dateStr] || []

    const close = currentCap
    const netChange = dayTxs.reduce((sum, t) => sum + t.amount, 0)
    const open = close - netChange

    let runningBalance = open
    let high = Math.max(open, close)
    let low = Math.min(open, close)
    let dayInflow = 0
    let dayOutflow = 0

    dayTxs.forEach(t => {
      if (t.amount > 0) dayInflow += t.amount
      else dayOutflow += Math.abs(t.amount)

      runningBalance += t.amount
      if (runningBalance > high) high = runningBalance
      if (runningBalance < low) low = runningBalance
    })

    result.unshift({
      date: dateStr,
      open,
      high,
      low,
      close,
      inflow: dayInflow,
      outflow: dayOutflow,
      isBullish: close >= open
    })

    currentCap = open
  }

  return result
})

const pxPerCandle = computed(() => Math.max(8, 22 * zoomLevel.value))

const svgWidth = computed(() => {
  const calculated = dailyCandles.value.length * pxPerCandle.value + paddingX * 2
  const minWidth = chartContainerRef.value ? chartContainerRef.value.clientWidth : 1000
  return Math.max(minWidth, calculated)
})

const candleScales = computed(() => {
  if (dailyCandles.value.length === 0) return { min: 0, max: 0, candles: [], ticks: [], monthBoundaries: [], step: 0 }

  const minVal = Math.min(...dailyCandles.value.map(c => c.low))
  const maxVal = Math.max(...dailyCandles.value.map(c => c.high))
  const range = maxVal - minVal || 1

  const count = dailyCandles.value.length
  const step = (svgWidth.value - paddingX * 2) / Math.max(count, 1)
  const candleWidth = Math.max(3, Math.min(24, step * 0.65))

  const getY = (val) => {
    return svgHeight - paddingY - ((val - minVal) / range) * (svgHeight - paddingY * 2)
  }

  const candles = dailyCandles.value.map((c, i) => {
    const x = paddingX + i * step + step / 2
    const yOpen = getY(c.open)
    const yClose = getY(c.close)
    const yHigh = getY(c.high)
    const yLow = getY(c.low)

    return {
      ...c,
      x,
      yHigh,
      yLow,
      bodyTop: Math.min(yOpen, yClose),
      bodyHeight: Math.max(2, Math.abs(yOpen - yClose)),
      width: candleWidth
    }
  })

  const monthBoundaries = []
  let lastMonth = null

  candles.forEach((c) => {
    const currentMonthKey = c.date.substring(0, 7)
    if (lastMonth && currentMonthKey !== lastMonth) {
      monthBoundaries.push(c)
    }
    lastMonth = currentMonthKey
  })

  const targetDistance = 80
  const candleStep = Math.max(1, Math.round(targetDistance / Math.max(1, step)))

  const ticks = []
  for (let i = 0; i < count; i += candleStep) {
    ticks.push(candles[i])
  }

  const lastCandle = candles[count - 1]
  if (ticks.length > 0 && ticks[ticks.length - 1].date !== lastCandle.date) {
    const distToLast = lastCandle.x - ticks[ticks.length - 1].x
    if (distToLast < targetDistance * 0.6) {
      ticks[ticks.length - 1] = lastCandle
    } else {
      ticks.push(lastCandle)
    }
  }

  return { min: minVal, max: maxVal, candles, ticks, monthBoundaries, step }
})

const handleWheel = (e) => {
  const delta = e.deltaY < 0 ? 0.15 : -0.15
  const newZoom = Math.min(4.0, Math.max(0.4, zoomLevel.value + delta))

  if (chartContainerRef.value) {
    const container = chartContainerRef.value
    const scrollRatio = (container.scrollLeft + container.clientWidth / 2) / container.scrollWidth
    zoomLevel.value = newZoom

    nextTick(() => {
      container.scrollLeft = scrollRatio * container.scrollWidth - container.clientWidth / 2
    })
  } else {
    zoomLevel.value = newZoom
  }
}

const resetZoom = () => {
  zoomLevel.value = 1.0
  scrollToEnd(true)
}

const scrollToEnd = (force = false) => {
  if (!force && hasInitialScrolled.value) return

  nextTick(() => {
    if (chartContainerRef.value) {
      chartContainerRef.value.scrollLeft = chartContainerRef.value.scrollWidth
      hasInitialScrolled.value = true
    }
  })
}

onMounted(() => {
  scrollToEnd()
})
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 p-5 rounded space-y-4">
    <div class="flex items-center justify-between text-xs border-b border-slate-800/80 pb-3">
      <div class="flex items-center gap-3">
        <span class="font-bold text-slate-300 uppercase tracking-wider">Динамика капитала</span>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">1D</span>
        <span class="text-[10px] text-slate-500">Дней: {{ candleScales.candles.length }}</span>
        <button 
          @click="resetZoom" 
          class="text-[10px] text-slate-400 hover:text-emerald-400 underline transition-colors"
        >
          Сброс ({{ Math.round(zoomLevel * 100) }}%)
        </button>
      </div>

      <div v-if="hoveredCandle" class="flex items-center gap-4 text-[11px] font-mono">
        <span class="text-slate-200 font-bold">{{ formatDateLong(hoveredCandle.date) }}</span>
        <span class="text-slate-500">Откр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.open) }}</strong></span>
        <span class="text-slate-500">Пик: <strong class="text-emerald-400">{{ formatMoney(hoveredCandle.high) }}</strong></span>
        <span class="text-slate-500">Мин: <strong class="text-rose-400">{{ formatMoney(hoveredCandle.low) }}</strong></span>
        <span class="text-slate-500">Закр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.close) }}</strong></span>
      </div>
      <div v-else class="text-[10px] text-slate-600 uppercase">Колесико — Zoom, Скролл — Навигация</div>
    </div>

    <div
      ref="chartContainerRef"
      @wheel.prevent="handleWheel"
      class="w-full overflow-x-auto scrollbar-thin scrollbar-thumb-slate-800 scrollbar-track-transparent"
    >
      <svg
        v-if="candleScales.candles.length > 0"
        :width="svgWidth"
        :height="svgHeight"
        class="select-none block"
      >
        <line x1="0" y1="30" :x2="svgWidth" y2="30" stroke="#1e293b" stroke-dasharray="3 3" />
        <line x1="0" y1="120" :x2="svgWidth" y2="120" stroke="#1e293b" stroke-dasharray="3 3" />
        <line x1="0" y1="210" :x2="svgWidth" y2="210" stroke="#1e293b" stroke-dasharray="3 3" />

        <line
          v-for="t in candleScales.ticks"
          :key="'v-grid-' + t.date"
          :x1="t.x"
          y1="10"
          :x2="t.x"
          :y2="svgHeight - 25"
          stroke="#1e293b"
          stroke-dasharray="2 2"
        />

        <g v-for="m in candleScales.monthBoundaries" :key="'m-boundary-' + m.date">
          <line
            :x1="m.x"
            y1="10"
            :x2="m.x"
            :y2="svgHeight - 25"
            stroke="#475569"
            stroke-width="1.5"
            stroke-dasharray="4 3"
          />
          <text
            :x="m.x + 4"
            y="20"
            fill="#94a3b8"
            font-size="9"
            font-weight="bold"
            font-family="monospace"
            class="uppercase select-none"
          >
            {{ formatMonthName(m.date) }}
          </text>
        </g>

        <line
          v-if="hoveredCandle"
          :x1="hoveredCandle.x"
          y1="10"
          :x2="hoveredCandle.x"
          :y2="svgHeight - 25"
          stroke="#64748b"
          stroke-width="1.5"
          stroke-dasharray="2 2"
        />

        <g
          v-for="c in candleScales.candles"
          :key="c.date"
          @mouseenter="hoveredCandle = c"
          @mouseleave="hoveredCandle = null"
          class="cursor-pointer"
        >
          <rect
            :x="c.x - candleScales.step / 2"
            y="0"
            :width="candleScales.step"
            :height="svgHeight"
            fill="transparent"
          />
          <line
            :x1="c.x"
            :y1="c.yHigh"
            :x2="c.x"
            :y2="c.yLow"
            :stroke="c.isBullish ? '#10b981' : '#f43f5e'"
            stroke-width="1.5"
            class="transition-opacity"
            :class="hoveredCandle && hoveredCandle.date === c.date ? 'opacity-100 stroke-2' : 'opacity-90'"
          />
          <rect
            :x="c.x - c.width / 2"
            :y="c.bodyTop"
            :width="c.width"
            :height="c.bodyHeight"
            :fill="c.isBullish ? '#10b981' : '#f43f5e'"
            rx="1"
            class="transition-opacity"
            :class="hoveredCandle && hoveredCandle.date === c.date ? 'opacity-100' : 'opacity-90'"
          />
        </g>

        <g>
          <text
            v-for="t in candleScales.ticks"
            :key="'tick-text-' + t.date"
            :x="t.x"
            :y="svgHeight - 6"
            text-anchor="middle"
            fill="#64748b"
            font-size="10"
            font-family="monospace"
          >
            {{ formatShortDateName(t.date) }}
          </text>
        </g>
      </svg>

      <div v-else class="h-48 flex items-center justify-center text-xs text-slate-600">
        Нет транзакций для построения свечей. Нажмите SYNC.
      </div>
    </div>
  </section>
</template>
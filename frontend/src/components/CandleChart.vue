<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
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

// Границы зума: ниже ~7% свеча уже тоньше 2px, дальше смысла нет. Раньше пол в 8px/свечу глушил отдаление на 40%.
const MIN_ZOOM = 0.07
const MAX_ZOOM = 4.0
const pxPerCandle = computed(() => Math.max(1.5, 22 * zoomLevel.value))

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
  const candleWidth = Math.max(1, Math.min(24, step * 0.65))

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

// Колесо — масштаб (якорь — свеча под курсором, как на биржах); горизонтальное колесо / Shift+колесо — сдвиг.
const handleWheel = (e) => {
  const container = chartContainerRef.value
  if (!container) return
  stopInertia()

  const horizontal = e.shiftKey || Math.abs(e.deltaX) > Math.abs(e.deltaY)
  if (horizontal) {
    container.scrollLeft += e.deltaX || e.deltaY
    return
  }

  // множитель вместо фиксированного шага: на малых значениях зум остаётся плавным
  const factor = e.deltaY < 0 ? 1.18 : 1 / 1.18
  const newZoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoomLevel.value * factor))
  if (newZoom === zoomLevel.value) return

  const rect = container.getBoundingClientRect()
  const mouseX = e.clientX - rect.left
  const ratio = (container.scrollLeft + mouseX) / container.scrollWidth
  zoomLevel.value = newZoom

  nextTick(() => {
    container.scrollLeft = ratio * container.scrollWidth - mouseX
  })
}

// ── Перетаскивание: вся область — «рукоятка», слайдера внизу нет (как на биржах). С инерцией после отпускания.
const isDragging = ref(false)
let dragStartX = 0
let dragStartScroll = 0
let lastX = 0
let lastT = 0
let velocity = 0 // px/мс
let inertiaRaf = 0

const stopInertia = () => {
  cancelAnimationFrame(inertiaRaf)
  inertiaRaf = 0
}

const onPointerDown = (e) => {
  if (e.pointerType === 'mouse' && e.button !== 0) return
  const container = chartContainerRef.value
  if (!container) return
  stopInertia()
  isDragging.value = true
  hoveredCandle.value = null
  dragStartX = lastX = e.clientX
  dragStartScroll = container.scrollLeft
  lastT = performance.now()
  velocity = 0
  container.setPointerCapture?.(e.pointerId)
}

const onPointerMove = (e) => {
  if (!isDragging.value) return
  const container = chartContainerRef.value
  container.scrollLeft = dragStartScroll - (e.clientX - dragStartX)

  const now = performance.now()
  const dt = now - lastT
  if (dt > 0) velocity = 0.8 * velocity + 0.2 * ((lastX - e.clientX) / dt)
  lastX = e.clientX
  lastT = now
}

const onPointerUp = (e) => {
  if (!isDragging.value) return
  isDragging.value = false
  chartContainerRef.value?.releasePointerCapture?.(e.pointerId)
  // палец/мышь остановились перед отпусканием — инерции нет
  if (performance.now() - lastT > 80) return

  let prev = performance.now()
  const step = (t) => {
    const container = chartContainerRef.value
    if (!container) return
    const dt = t - prev
    prev = t
    container.scrollLeft += velocity * dt
    velocity *= Math.pow(0.95, dt / 16)
    if (Math.abs(velocity) > 0.02) inertiaRaf = requestAnimationFrame(step)
  }
  inertiaRaf = requestAnimationFrame(step)
}

onBeforeUnmount(stopInertia)

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

// Данные приходят после монтирования: когда свечи впервые появились — встаём на свежий край
let scrolledWithData = false
watch(() => dailyCandles.value.length, (n) => {
  if (n > 0 && !scrolledWithData) {
    scrolledWithData = true
    scrollToEnd(true)
  }
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
      <div v-else class="text-[10px] text-slate-600 uppercase">Колесико — Zoom, Перетаскивание — Навигация</div>
    </div>

    <div
      ref="chartContainerRef"
      data-wheel="own"
      @wheel.prevent="handleWheel"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      class="chart-pan w-full overflow-x-auto"
      :class="isDragging ? 'cursor-grabbing' : 'cursor-grab'"
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
          @mouseenter="!isDragging && (hoveredCandle = c)"
          @mouseleave="hoveredCandle = null"
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

<style scoped>
/* Слайдера внизу нет: двигаем перетаскиванием. Полосу прокрутки прячем, жест pan-y оставляет вертикальный скролл страницы на таче. */
.chart-pan {
  scrollbar-width: none;
  touch-action: pan-y;
  user-select: none;
}
.chart-pan::-webkit-scrollbar {
  display: none;
}
</style>

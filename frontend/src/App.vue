<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'

const API_BASE = 'http://localhost:8080/api'

const activeTab = ref('dashboard')
const accounts = ref([])
const transactions = ref([])
const syncStats = ref({ total_transactions: 0, oldest_tx_time: '—', newest_tx_time: '—' })
const isSyncing = ref(false)
const syncStatusMsg = ref('IDLE')
const errorMsg = ref('')

const hoveredCandle = ref(null)
const chartContainerRef = ref(null)
const zoomLevel = ref(1.0)
const hasInitialScrolled = ref(false)

const editingLimitId = ref(null)
const editLimitValue = ref(0)

let pollTimer = null

const now = new Date()
const currentYear = now.getFullYear()
const currentMonth = now.getMonth()

const toLocalDateStr = (d) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const formatMoney = (val) => {
  return new Intl.NumberFormat('uk-UA', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(val || 0)
}

const formatDateLong = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]))
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

const formatShortDateName = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]))
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

const formatMonthName = (dateStr) => {
  if (!dateStr) return ''
  const parts = dateStr.split('-')
  if (parts.length !== 3) return dateStr
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, 1)
  return d.toLocaleDateString('ru-RU', { month: 'short' }).toUpperCase()
}

const currentMonthTransactions = computed(() => {
  return transactions.value.filter(t => {
    if (!t.time) return false
    const d = new Date(t.time)
    return d.getFullYear() === currentYear && d.getMonth() === currentMonth
  })
})

const totalValueUAH = computed(() => {
  return accounts.value
    .filter(a => a.currency_code === 980)
    .reduce((sum, a) => sum + a.balance, 0)
})

const monthlyInflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount > 0)
    .reduce((sum, t) => sum + t.amount, 0)
})

const monthlyOutflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount < 0)
    .reduce((sum, t) => sum + Math.abs(t.amount), 0)
})

const monthlyDelta = computed(() => {
  return monthlyInflow.value - monthlyOutflow.value
})

// Точное построение свечей с точной воспроизводимостью внутридневного баланса (Intraday Replay)
const dailyCandles = computed(() => {
  if (transactions.value.length === 0) return []

  const txByDate = {}
  let minDateObj = new Date()

  // 1. Сортируем транзакции строго по времени
  const sortedTx = [...transactions.value]
    .filter(t => t.time)
    .sort((a, b) => new Date(a.time) - new Date(b.time))

  sortedTx.forEach(t => {
    const dObj = new Date(t.time)
    if (dObj < minDateObj) minDateObj = dObj
    const dStr = toLocalDateStr(dObj)

    if (!txByDate[dStr]) txByDate[dStr] = []
    txByDate[dStr].push(t)
  })

  // 2. Формируем непрерывный календарный ряд
  const allDays = []
  const curr = new Date(minDateObj)
  curr.setHours(0, 0, 0, 0)
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  while (curr <= today) {
    allDays.push(toLocalDateStr(curr))
    curr.setDate(curr.getDate() + 1)
  }

  // 3. Отматываем капитал назад с пошаговой симуляцией хронологии транзакций дня
  let currentCap = totalValueUAH.value
  const result = []

  for (let i = allDays.length - 1; i >= 0; i--) {
    const dateStr = allDays[i]
    const dayTxs = txByDate[dateStr] || []

    const close = currentCap

    // Суммарное изменение баланса за день
    const netChange = dayTxs.reduce((sum, t) => sum + t.amount, 0)
    const open = close - netChange

    // Хронологический прогон баланса для вычисления НАСТОЯЩИХ High и Low
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

const svgHeight = 260
const paddingY = 30
const paddingX = 30

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

    const bodyTop = Math.min(yOpen, yClose)
    const bodyHeight = Math.max(2, Math.abs(yOpen - yClose))

    return {
      ...c,
      x,
      yHigh,
      yLow,
      bodyTop,
      bodyHeight,
      width: candleWidth
    }
  })

  // Разделители между календарными месяцами
  const monthBoundaries = []
  let lastMonth = null

  candles.forEach((c) => {
    const currentMonthKey = c.date.substring(0, 7)
    if (lastMonth && currentMonthKey !== lastMonth) {
      monthBoundaries.push(c)
    }
    lastMonth = currentMonthKey
  })

  // Равномерный расчёт вертикальной сетки без выпадающих линий
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

const startEditLimit = (acc) => {
  editingLimitId.value = acc.id
  editLimitValue.value = acc.credit_limit
}

const cancelEditLimit = () => {
  editingLimitId.value = null
}

const saveLimit = async (accId) => {
  try {
    const res = await fetch(`${API_BASE}/accounts/credit-limit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        account_id: accId,
        credit_limit: parseFloat(editLimitValue.value) || 0
      })
    })
    if (!res.ok) throw new Error('Ошибка сохранения лимита')
    editingLimitId.value = null
    await fetchData()
  } catch (err) {
    errorMsg.value = err.message
  }
}

const fetchStatus = async () => {
  try {
    const res = await fetch(`${API_BASE}/sync/status`)
    if (res.ok) {
      const data = await res.json()
      isSyncing.value = data.running
      syncStatusMsg.value = data.status
      if (data.stats) syncStats.value = data.stats
    }
  } catch (err) {
    console.error('Status fetch error:', err)
  }
}

const fetchData = async () => {
  try {
    const [accRes, txRes] = await Promise.all([
      fetch(`${API_BASE}/accounts`),
      fetch(`${API_BASE}/transactions`)
    ])
    if (accRes.ok) accounts.value = (await accRes.json()) || []
    if (txRes.ok) {
      transactions.value = (await txRes.json()) || []
      if (!hasInitialScrolled.value) {
        scrollToEnd()
      }
    }
    await fetchStatus()
  } catch (err) {
    errorMsg.value = err.message
  }
}

const startFullSync = async () => {
  errorMsg.value = ''
  try {
    const res = await fetch(`${API_BASE}/sync/full`, { method: 'POST' })
    if (!res.ok) throw new Error(await res.text())
    await fetchStatus()
  } catch (err) {
    errorMsg.value = err.message
  }
}

onMounted(() => {
  fetchData()
  pollTimer = setInterval(() => {
    fetchStatus()
    fetchData()
  }, 3000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 font-mono flex flex-col p-6">
    <div class="max-w-6xl w-full mx-auto space-y-8">
      
      <!-- Terminal Header Menu -->
      <header class="border-b border-slate-800 pb-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="flex items-center gap-6">
          <div class="flex items-center gap-2">
            <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" :class="{ 'animate-pulse': isSyncing }"></span>
            <span class="text-sm font-bold tracking-wider text-white">ECONOMIC</span>
          </div>

          <nav class="flex items-center gap-1 border-l border-slate-800 pl-6 text-xs">
            <button
              @click="activeTab = 'dashboard'"
              class="px-3 py-1.5 rounded transition-colors uppercase font-bold"
              :class="activeTab === 'dashboard' ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
            >
              Дашборд
            </button>
            <button
              @click="activeTab = 'accounts'"
              class="px-3 py-1.5 rounded transition-colors uppercase font-bold"
              :class="activeTab === 'accounts' ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
            >
              Операционный блок
            </button>
          </nav>
        </div>

        <div class="flex items-center gap-4">
          <span class="text-[11px] text-slate-500 uppercase">{{ syncStatusMsg }}</span>
          <button
            @click="startFullSync"
            :disabled="isSyncing"
            class="bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-800 disabled:text-slate-600 text-white font-bold text-xs px-4 py-1.5 rounded transition-all cursor-pointer uppercase"
          >
            {{ isSyncing ? 'SYNCING...' : 'SYNC' }}
          </button>
        </div>
      </header>

      <!-- Alert -->
      <div v-if="errorMsg" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">
        ERROR: {{ errorMsg }}
      </div>

      <!-- VIEW 1: DASHBOARD -->
      <main v-if="activeTab === 'dashboard'" class="space-y-6">
        
        <!-- Total Capital Card -->
        <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="bg-slate-900/60 border border-slate-800 p-5 rounded space-y-2">
            <span class="text-[10px] text-slate-500 uppercase tracking-wider block">Total Capital</span>
            <div class="flex flex-wrap items-baseline gap-3">
              <span class="text-2xl font-bold text-white">{{ formatMoney(totalValueUAH) }} ₴</span>
              <span
                class="text-xs font-bold px-2 py-0.5 rounded border"
                :class="monthlyDelta >= 0 ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
              >
                {{ monthlyDelta >= 0 ? '+' : '' }}{{ formatMoney(monthlyDelta) }} ₴ <span class="text-[10px] font-normal opacity-75">MTD</span>
              </span>
            </div>
          </div>
        </section>

        <!-- 1D Candlestick Chart Widget -->
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

            <!-- Inspect Tooltip -->
            <div v-if="hoveredCandle" class="flex items-center gap-4 text-[11px] font-mono">
              <span class="text-slate-200 font-bold">{{ formatDateLong(hoveredCandle.date) }}</span>
              <span class="text-slate-500">Откр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.open) }}</strong></span>
              <span class="text-slate-500">Пик: <strong class="text-emerald-400">{{ formatMoney(hoveredCandle.high) }}</strong></span>
              <span class="text-slate-500">Мин: <strong class="text-rose-400">{{ formatMoney(hoveredCandle.low) }}</strong></span>
              <span class="text-slate-500">Закр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.close) }}</strong></span>
            </div>
            <div v-else class="text-[10px] text-slate-600 uppercase">Колесико — Zoom, Скролл — Навигация</div>
          </div>

          <!-- Canvas Container -->
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
              <!-- Horizontal Grid Lines -->
              <line x1="0" y1="30" :x2="svgWidth" y2="30" stroke="#1e293b" stroke-dasharray="3 3" />
              <line x1="0" y1="120" :x2="svgWidth" y2="120" stroke="#1e293b" stroke-dasharray="3 3" />
              <line x1="0" y1="210" :x2="svgWidth" y2="210" stroke="#1e293b" stroke-dasharray="3 3" />

              <!-- Regular Date Grid Lines -->
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

              <!-- Distinct Month Boundary Separators -->
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

              <!-- Hover Crosshair Line -->
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

              <!-- Candlesticks & Full-Column Interactive Hitboxes -->
              <g
                v-for="c in candleScales.candles"
                :key="c.date"
                @mouseenter="hoveredCandle = c"
                @mouseleave="hoveredCandle = null"
                class="cursor-pointer"
              >
                <!-- Full Column Hitbox -->
                <rect
                  :x="c.x - candleScales.step / 2"
                  y="0"
                  :width="candleScales.step"
                  :height="svgHeight"
                  fill="transparent"
                />

                <!-- Wick -->
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

                <!-- Body -->
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

              <!-- X-Axis Labels -->
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

      </main>

      <!-- VIEW 2: OPERATIONAL BLOCK -->
      <main v-if="activeTab === 'accounts'" class="space-y-4">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Подключенные счета и балансы</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="acc in accounts"
            :key="acc.id"
            class="bg-slate-900/60 border border-slate-800 p-5 rounded flex flex-col justify-between"
          >
            <div>
              <div class="text-sm font-bold text-slate-200 truncate">{{ acc.name }}</div>
              <div class="text-[10px] text-slate-500 uppercase mt-1">ТИП: {{ acc.type }}</div>
            </div>
            
            <div class="mt-6 space-y-2">
              <div>
                <span class="text-[10px] text-slate-500 uppercase block">Баланс счета:</span>
                <span class="text-xl font-bold text-white">{{ formatMoney(acc.balance) }} ₴</span>
              </div>

              <div class="border-t border-slate-800/80 pt-2 text-[11px]">
                <div v-if="editingLimitId === acc.id" class="flex items-center gap-2 mt-1">
                  <input
                    v-model.number="editLimitValue"
                    type="number"
                    step="100"
                    class="bg-slate-950 border border-slate-700 text-xs px-2 py-1 rounded text-white w-28 outline-none focus:border-emerald-500"
                  />
                  <button
                    @click="saveLimit(acc.id)"
                    class="bg-emerald-600 hover:bg-emerald-500 text-white text-[10px] font-bold px-2 py-1 rounded uppercase"
                  >
                    ОК
                  </button>
                  <button
                    @click="cancelEditLimit"
                    class="text-slate-500 hover:text-slate-300 text-[10px] uppercase"
                  >
                    Отмена
                  </button>
                </div>

                <div v-else class="flex items-center justify-between text-slate-400 mt-1">
                  <span>Кредитный лимит: <strong class="text-slate-300">{{ formatMoney(acc.credit_limit) }} ₴</strong></span>
                  <button
                    @click="startEditLimit(acc)"
                    class="text-[10px] text-emerald-400 hover:underline uppercase font-bold"
                  >
                    Изм.
                  </button>
                </div>
              </div>
            </div>

          </div>
        </div>
      </main>

    </div>
  </div>
</template>
<script setup>
import { ref, computed, nextTick, onMounted, onActivated, onDeactivated, onBeforeUnmount, watch } from 'vue'
import { apiSend } from '../utils/api.js'
import { storageGet, storageSet } from '../utils/storage.js'
import { TFS, dayStartOf, weekStartOf, alignDown, buildIntradayStarts, buildDailyStarts, lowerBound, niceStep, axisLabel } from '../utils/candleMath.js'
import { toLocalDateStr, formatMoney, formatDateLong, formatShortDateName, formatMonthName } from '../utils/formatters.js'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  totalValueUAH: { type: Number, default: 0 },
  // счета Binance: пока минутного ряда оценки нет, их операции считаются как у любого счёта; с рядом — их скачок уже в оценке
  binanceIds: { type: Array, default: () => [] },
  // выплаты по облигациям инвестиционных счетов: { date: 'ГГГГ-ММ-ДД', kind: coupon|redemption, amount, isin, done }
  events: { type: Array, default: () => [] },
  // клик по свече выбирает её (наружу — событие select, на графике подсветка); без флага график только смотрят
  selectable: { type: Boolean, default: false }
})

const emit = defineEmits(['select'])
const hoveredCandle = ref(null)
const selectedKey = ref(null) // выбранная кликом свеча (только при selectable): наружу уходят границы её корзины
const hoveredMarker = ref(null)
const BLUE = '#93c5fd' // голубой как у бейджа покупки ОВДП
const CLAIMED = '#64748b' // выплата уже получена — серым
const chartContainerRef = ref(null)
const zoomLevel = ref(1.0)
const hasInitialScrolled = ref(false)

const svgHeight = 260
const paddingY = 30
const paddingX = 30

// ── Таймфреймы. Внутридневные окна ограничены (минутных свечей за годы — сотни тысяч), 1D и 1W — вся история.
const readTf = () => {
  const v = storageGet('zoe.candle.tf')
  return TFS.some(t => t.id === v) ? v : '1D'
}
const tfId = ref(readTf())
const tfDef = computed(() => TFS.find(t => t.id === tfId.value) || TFS[5])
const isDaily = computed(() => !!(tfDef.value.day || tfDef.value.week))
const pad2 = (n) => String(n).padStart(2, '0')
const setTf = (id) => {
  if (id === tfId.value) return
  tfId.value = id
  extraBack.value = 0
  storageSet('zoe.candle.tf', id)
  zoomLevel.value = 1.0
  hoveredCandle.value = null
  selectCandle(null)
  nextTick(() => scrollToEnd(true))
}


// ── Ряд оценки Binance. Свечи подгружаются чанками и копятся в кеше по началу корзины (мс → [o,h,l,c]) отдельно для каждого
// таймфрейма: возврат на уже открытый ТФ и прокрутка назад не ходят в бэкенд за тем, что уже есть.
const bnInfo = ref({ has: false, first: 0, last: 0 })
// ряд оценки участвует в графике, только если Binance среди выбранных счетов
const bnHas = computed(() => props.binanceIds.length > 0 && bnInfo.value.has)
const ZERO = [0, 0, 0, 0]
const bnCaches = {}
const bnCache = () => (bnCaches[tfId.value] ||= new Map())
let bnLastValue = 0 // последняя точка ряда: от неё график считает «остальной капитал»
const bnVer = ref(0) // растёт при каждом пополнении кеша → пересчёт свечей
const bnReady = ref(false) // свечи всего текущего окна уже в кеше: операции Binance-счетов не считаем второй раз
let bnEpoch = 0 // смена ряда сбрасывает кеши; ответы запросов, ушедших до этого, выбрасываются
const resetBn = () => {
  bnEpoch++
  for (const k of Object.keys(bnCaches)) delete bnCaches[k]
  bnReady.value = false
  bnVer.value++
}

// Окно: DST-безопасно и без Date на каждую свечу — день → его корзины арифметикой (сотни тысяч минутных свечей не тормозят).
// Самая ранняя точка графика: первая операция или начало ряда Binance
const minTimeMs = computed(() => {
  let m = null
  for (const t of props.transactions) {
    if (!t.time) continue
    const v = new Date(t.time).getTime()
    if (m === null || v < m) m = v
  }
  if (bnHas.value && bnInfo.value.first) {
    const f = bnInfo.value.first * 1000
    if (m === null || f < m) m = f
  }
  return m
})

// Внутридневное окно растёт чанками (extraBack) при прокрутке к левому краю; 1D и 1W — вся история сразу (их мало).
const extraBack = ref(0)
const startsFor = (extra) => {
  const minT = minTimeMs.value
  if (minT === null) return []
  const def = tfDef.value
  return def.day || def.week ? buildDailyStarts(def, minT) : buildIntradayStarts(def.ms, minT, def.limit + extra)
}
const bucketStarts = computed(() => startsFor(extraBack.value))
const canLoadMore = computed(() => !isDaily.value && bucketStarts.value.length > 0 && minTimeMs.value !== null && bucketStarts.value[0] > minTimeMs.value)

// Корзина i — [starts[i], starts[i+1]); операция «в будущем» (например, 12:00 сегодняшнего дня) попадает в последнюю

// Свечи Binance под корзины, которых ещё нет в кеше: непрерывными кусками, один запрос на кусок.
const fetchBnRun = async (starts, i0, i1, epoch, tf) => {
  const slice = starts.slice(i0, i1 + 1)
  const end = i1 + 1 < starts.length ? starts[i1 + 1] : Date.now() + 60_000
  const res = await apiSend('POST', '/binance/candles', { starts: slice.map(v => Math.floor(v / 1000)), end: Math.floor(end / 1000) })
  const d = await res.json()
  if (epoch !== bnEpoch || tf !== tfId.value) return false
  if (!d.has) return false
  bnLastValue = d.last_value || 0
  const cache = bnCaches[tf] ||= new Map()
  slice.forEach((s, k) => cache.set(s, (d.candles && d.candles[k]) || ZERO))
  return true
}
let bnBusy = 0
let bnChain = Promise.resolve() // запуски идут по очереди: второй видит кеш первого и не дублирует запрос
const ensureBn = () => (bnChain = bnChain.then(runEnsureBn, runEnsureBn))
const runEnsureBn = async () => {
  const starts = bucketStarts.value
  if (!bnHas.value || !starts.length) { bnReady.value = false; return }
  const my = ++bnBusy
  const epoch = bnEpoch
  const tf = tfId.value
  const cache = bnCache()
  try {
    let i = 0
    while (i < starts.length) {
      if (cache.has(starts[i])) { i++; continue }
      let j = i
      while (j + 1 < starts.length && !cache.has(starts[j + 1])) j++
      if (!(await fetchBnRun(starts, i, j, epoch, tf))) return
      bnVer.value++
      i = j + 1
    }
    if (my === bnBusy && epoch === bnEpoch) bnReady.value = true
  } catch { /* связь пропала — повторим при следующем обновлении данных */ }
}

// Что поменялось в ряду: другое начало/ужатие — кеши недействительны; дописан хвост — сбрасываем только корзины от старого края.
let infoFlight = null // одновременные вызовы делят один запрос
const loadBnInfo = () => (infoFlight ||= fetchBnInfo().finally(() => { infoFlight = null }))
const fetchBnInfo = async () => {
  let next
  try {
    const res = await apiSend('POST', '/binance/candles', { starts: [], end: 0 })
    const d = await res.json()
    next = { has: !!d.has, first: d.first || 0, last: d.last || 0 }
    bnLastValue = d.last_value || 0
  } catch { next = { has: false, first: 0, last: 0 } }
  const prev = bnInfo.value
  bnInfo.value = next
  if (!next.has || next.first !== prev.first || next.last < prev.last) resetBn()
  else if (next.last !== prev.last) {
    const cut = prev.last * 1000
    for (const cache of Object.values(bnCaches)) {
      let from = -Infinity // корзина, в которую попадал старый край ряда, и всё правее
      for (const k of cache.keys()) if (k <= cut && k > from) from = k
      for (const k of [...cache.keys()]) if (k >= from) cache.delete(k)
    }
    bnReady.value = false
    bnVer.value++
  }
}
watch(() => props.transactions, async () => { await loadBnInfo(); ensureBn() })
watch([bucketStarts, bnHas], ensureBn)
onMounted(async () => { await loadBnInfo(); ensureBn() })

// Операции (кроме Binance-счетов при готовом ряде), по времени. Типизированные массивы: пересчёт свечей без аллокаций.
const txSorted = computed(() => {
  const skip = bnReady.value ? new Set(props.binanceIds) : null
  const list = []
  for (const t of props.transactions) {
    if (!t.time || (skip && skip.has(t.account_id))) continue
    list.push([new Date(t.time).getTime(), t.amount])
  }
  list.sort((a, b) => a[0] - b[0])
  const ms = new Float64Array(list.length)
  const amt = new Float64Array(list.length)
  list.forEach((x, i) => { ms[i] = x[0]; amt[i] = x[1] })
  return { ms, amt }
})

// Все загруженные корзины в плоских массивах. Считается справа налево от итога капитала (как раньше), без объекта на свечу —
// объекты строятся только для видимого окна.
const series = computed(() => {
  const starts = bucketStarts.value
  const n = starts.length
  if (n === 0) return null
  void bnVer.value
  const { ms, amt } = txSorted.value
  const lo = new Int32Array(n + 1)
  let p = 0
  while (p < ms.length && ms[p] < starts[0]) p++
  for (let i = 0; i < n; i++) {
    lo[i] = p
    const end = i + 1 < n ? starts[i + 1] : Infinity
    while (p < ms.length && ms[p] < end) p++
  }
  lo[n] = p

  const bn = bnReady.value ? bnCache() : null
  const O = new Float64Array(n), H = new Float64Array(n), L = new Float64Array(n), C = new Float64Array(n)
  const BO = new Float64Array(n), BC = new Float64Array(n), IN = new Float64Array(n), OUT = new Float64Array(n)
  let cur = props.totalValueUAH - (bn ? bnLastValue : 0)
  let minVal = Infinity, maxVal = -Infinity
  for (let i = n - 1; i >= 0; i--) {
    const c1 = cur
    let net = 0
    for (let k = lo[i]; k < lo[i + 1]; k++) net += amt[k]
    const o1 = c1 - net
    let run = o1
    let h1 = Math.max(o1, c1)
    let l1 = Math.min(o1, c1)
    let inflow = 0, outflow = 0
    for (let k = lo[i]; k < lo[i + 1]; k++) {
      const a = amt[k]
      if (a > 0) inflow += a; else outflow -= a
      run += a
      if (run > h1) h1 = run
      if (run < l1) l1 = run
    }
    cur = o1
    // Binance: свеча оценки (o,h,l,c); до начала ряда — ноль. Пути двух частей внутри корзины неизвестны —
    // пик/дно берём из достижимых сочетаний крайних точек.
    const b = (bn && bn.get(starts[i])) || ZERO
    const open = o1 + b[0]
    const close = c1 + b[3]
    const high = Math.max(open, close, h1 + b[0], h1 + b[3], o1 + b[1], c1 + b[1])
    const low = Math.min(open, close, l1 + b[0], l1 + b[3], o1 + b[2], c1 + b[2])
    O[i] = open; H[i] = high; L[i] = low; C[i] = close
    BO[i] = b[0]; BC[i] = b[3]; IN[i] = inflow; OUT[i] = outflow
    if (low < minVal) minVal = low
    if (high > maxVal) maxVal = high
  }
  return { n, starts, O, H, L, C, BO, BC, IN, OUT, min: minVal, max: maxVal, bnSeries: !!bn }
})

// Таймлайн вперёд — до последней выплаты инвестиционного счёта (погашение). Свечей там нет, только маркеры. Только на 1D.
const futureDays = computed(() => {
  const weekly = tfId.value === '1W'
  const todayStr = toLocalDateStr(new Date())
  const last = props.events.reduce((m, e) => (e.date > m ? e.date : m), todayStr)
  const days = []
  const curr = new Date(weekly ? weekStartOf(Date.now()) : dayStartOf(Date.now()))
  curr.setDate(curr.getDate() + (weekly ? 7 : 1))
  while (toLocalDateStr(curr) <= last && days.length < 4000) {
    days.push(toLocalDateStr(curr))
    curr.setDate(curr.getDate() + (weekly ? 7 : 1))
  }
  return days
})
// Выплата → дата её слота: на 1W слот недели (понедельник), иначе сам день
const slotDateOf = (dateStr) => {
  if (tfId.value !== '1W') return dateStr
  const [y, m, d] = dateStr.split('-').map(Number)
  return toLocalDateStr(new Date(weekStartOf(new Date(y, m - 1, d).getTime())))
}
const totalDays = computed(() => (series.value ? series.value.n + futureDays.value.length : 0))

// Границы зума: ниже ~7% свеча уже тоньше 2px, дальше смысла нет. Раньше пол в 8px/свечу глушил отдаление на 40%.
const MIN_ZOOM = 0.07
const MAX_ZOOM = 4.0
const pxPerCandle = computed(() => Math.max(1.5, 22 * zoomLevel.value))

const svgWidth = computed(() => {
  const calculated = totalDays.value * pxPerCandle.value + paddingX * 2
  const minWidth = chartContainerRef.value ? chartContainerRef.value.clientWidth : 1000
  return Math.max(minWidth, calculated)
})
const stepPx = computed(() => (svgWidth.value - paddingX * 2) / Math.max(totalDays.value, 1))

// ── Окно отрисовки. Рисуем только свечи на экране и по экрану запаса с каждой стороны: DOM не растёт с историей.
// Окно двигается квантами (полэкрана): пока двигаешь график внутри кванта, Vue ничего не перерисовывает.
const scrollX = ref(0)
const viewW = ref(1000)
const perView = computed(() => Math.max(1, Math.ceil(viewW.value / stepPx.value)))
const quant = computed(() => Math.max(8, Math.ceil(perView.value / 2)))
const firstVisible = computed(() => Math.floor((scrollX.value - paddingX) / stepPx.value))
const winFrom = computed(() => Math.max(0, Math.floor((firstVisible.value - perView.value) / quant.value) * quant.value))
const winTo = computed(() => Math.min(totalDays.value, Math.ceil((firstVisible.value + perView.value * 2) / quant.value) * quant.value))


// ── Вертикальная шкала по видимой части (как на биржах), а не по всей загруженной истории:
// подгрузка чанков с другими экстремумами её не трогает. Цель — min/max свечей на экране + поля;
// показываемый диапазон плавно (экспоненциально, по времени) догоняет цель, поэтому смена масштаба при прокрутке не дёргается.
const yLo = ref(0), yHi = ref(1)
const yTarget = computed(() => {
  const S = series.value
  if (!S || !S.n) return null
  const a = Math.max(0, firstVisible.value)
  const b = Math.min(S.n, firstVisible.value + perView.value + 1)
  let lo = Infinity, hi = -Infinity
  for (let i = a; i < b; i++) { if (S.L[i] < lo) lo = S.L[i]; if (S.H[i] > hi) hi = S.H[i] }
  if (lo === Infinity) { lo = S.min; hi = S.max } // экран целиком в будущем/пусто — держим общий диапазон
  const pad = (hi - lo) * 0.04 || Math.abs(hi) * 0.01 || 1
  return [lo - pad, hi + pad]
})
let yRaf = 0, yLast = 0, yInit = false
const yStep = (t) => {
  yRaf = 0
  const tg = yTarget.value
  if (!tg) return
  const dt = Math.min(100, t - yLast || 16)
  yLast = t
  const k = 1 - Math.exp(-dt / 110)
  const span = tg[1] - tg[0] || 1
  const dLo = tg[0] - yLo.value, dHi = tg[1] - yHi.value
  if (Math.abs(dLo) < span * 0.0005 && Math.abs(dHi) < span * 0.0005) { yLo.value = tg[0]; yHi.value = tg[1]; return }
  yLo.value += dLo * k
  yHi.value += dHi * k
  yRaf = requestAnimationFrame(yStep)
}
watch(yTarget, (tg) => {
  if (!tg) return
  if (!yInit) { yInit = true; yLo.value = tg[0]; yHi.value = tg[1]; return } // первый показ — без анимации
  if (!yRaf) { yLast = performance.now(); yRaf = requestAnimationFrame(yStep) }
}, { immediate: true })
onBeforeUnmount(() => { if (yRaf) cancelAnimationFrame(yRaf) })

const slotMeta = (i) => {
  const S = series.value
  if (i < S.n) {
    const d = new Date(S.starts[i])
    return { date: toLocalDateStr(d), time: `${pad2(d.getHours())}:${pad2(d.getMinutes())}`, future: false }
  }
  return { date: futureDays.value[i - S.n], time: '', future: true }
}
const slotX = (i) => paddingX + i * stepPx.value + stepPx.value / 2

// Выплата → слоты: по дате; в нескольких слотах одного дня (внутридневные ТФ) маркер ставим на первом
const eventsByDate = computed(() => {
  const m = new Map()
  props.events.forEach(e => {
    const d = slotDateOf(e.date)
    if (!m.has(d)) m.set(d, [])
    m.get(d).push(e)
  })
  return m
})

// ── Шкала цены справа (как на бирже): «круглые» отметки по текущему окну, сетка по ним, тег последней цены.
const yOf = (val) => {
  const range = yHi.value - yLo.value || 1
  return svgHeight - paddingY - ((val - yLo.value) / range) * (svgHeight - paddingY * 2)
}
const priceTicks = computed(() => {
  const lo = yLo.value, hi = yHi.value
  if (!(hi > lo)) return []
  const step = niceStep((hi - lo) / 6)
  const out = []
  for (let v = Math.ceil(lo / step) * step; v <= hi + step * 1e-9; v += step) {
    const val = Math.round(v / step) * step
    out.push({ val, y: yOf(val), label: axisLabel(val, step, lo, hi) })
  }
  return out
})
// компактная подпись в одной единице на всю шкалу (9,5к · 10к · 10,5к), мелкие значения — как есть
// тег последней цены — точное число без сокращений («10 150»); сокращать только от миллиона, иначе тег не влезает
const lastLabel = computed(() => {
  const v = lastPrice.value ? lastPrice.value.val : 0
  return Math.abs(v) >= 1e6 ? (v / 1e6).toFixed(2).replace('.', ',') + 'М' : new Intl.NumberFormat('uk-UA', { maximumFractionDigits: 0 }).format(v)
})
const lastPrice = computed(() => {
  const S = series.value
  if (!S || !S.n) return null
  const val = S.C[S.n - 1]
  const up = S.C[S.n - 1] >= S.O[S.n - 1]
  return { val, up, y: Math.min(svgHeight - paddingY, Math.max(paddingY / 2, yOf(val))) }
})

const candleAt = (i) => {
  const S = series.value
  const min = yLo.value, max = yHi.value
  const range = max - min || 1
  const getY = (val) => svgHeight - paddingY - ((val - min) / range) * (svgHeight - paddingY * 2)
  const step = stepPx.value
  const yOpen = getY(S.O[i]), yClose = getY(S.C[i])
  const m = slotMeta(i)
  return {
    key: String(S.starts[i]), i, ts: S.starts[i], date: m.date, time: m.time,
    open: S.O[i], high: S.H[i], low: S.L[i], close: S.C[i],
    bnOpen: S.BO[i], bnClose: S.BC[i], bnSeries: S.bnSeries, // вклад Binance в открытие/закрытие (для разбора свечи)
    inflow: S.IN[i], outflow: S.OUT[i],
    isBullish: S.C[i] >= S.O[i],
    x: slotX(i), yHigh: getY(S.H[i]), yLow: getY(S.L[i]),
    bodyTop: Math.min(yOpen, yClose), bodyHeight: Math.max(2, Math.abs(yOpen - yClose)),
    width: Math.max(1, Math.min(24, step * 0.65))
  }
}

// Видимая часть шкалы: свечи окна, сетка, границы периодов, маркеры выплат
const candleScales = computed(() => {
  const S = series.value
  const empty = { candles: [], ticks: [], monthBoundaries: [], markers: [], step: 0, count: 0 }
  if (!S) return empty
  const a = winFrom.value, b = winTo.value
  const step = stepPx.value
  const nCandles = S.n

  const candles = []
  for (let i = a; i < Math.min(b, nCandles); i++) candles.push(candleAt(i))

  // маркеры выплат лежат на «полу» таймлайна, прямо над датами; в один день сыплются столбиком вверх (только на 1D)
  const floorY = svgHeight - 38
  const slotOfDate = new Map()
  const slots = []
  for (let i = a; i < b; i++) {
    const m = slotMeta(i)
    const sl = i < nCandles ? candles[i - a] : { key: 'f' + m.date, date: m.date, future: true, x: slotX(i) }
    slots.push(sl)
    if (!slotOfDate.has(m.date)) slotOfDate.set(m.date, sl)
  }
  const markers = []
  eventsByDate.value.forEach((list, date) => {
    const slot = slotOfDate.get(date)
    if (!slot) return
    // по маркеру на каждую бумагу (3 облигации — 3 иконки). Купоны и погашения — в разных колонках рядом, друг на друга не ложатся
    const COL = 9, MAX_STACK = 6, GAP = 15
    const kinds = ['coupon', 'redemption'].filter(k => list.some(e => e.kind === k))
    kinds.forEach((kind, ki) => {
      const units = []
      list.filter(e => e.kind === kind).sort((x, y) => x.isin.localeCompare(y.isin)).forEach(e => {
        const n = Math.max(1, Math.round(e.qty || 1))
        for (let u = 0; u < n; u++) units.push({ ...e, amount: e.amount / n, unit: u + 1, of: n })
      })
      const colX = slot.x + (kinds.length === 2 ? (ki === 0 ? -COL : COL) : 0)
      units.slice(0, MAX_STACK).forEach((e, i) => markers.push({ ...e, key: `${date}-${kind}-${e.isin}-${e.unit}`, x: colX, y: floorY - i * GAP }))
      // больше 6 — правее верхней иконки подпись xN в цвет иконок
      if (units.length > MAX_STACK) {
        const top = units[0]
        markers.push({ more: true, key: `${date}-${kind}-more`, x: colX + 10, y: floorY - (MAX_STACK - 1) * GAP + 3, n: units.length, done: top.done, color: top.color })
      }
    })
  })

  // границы: на дневных таймфреймах — месяцы, на внутридневных — сутки; в будущем (слоты-дни) — месяцы
  const monthBoundaries = []
  const periodKey = (m) => (isDaily.value || m.future ? m.date.substring(0, 7) : m.date)
  let prevM = a > 0 ? slotMeta(a - 1) : null
  for (let i = a; i < b; i++) {
    const m = slotMeta(i)
    const changed = m.future && !isDaily.value
      ? (prevM ? m.date.substring(0, 7) !== prevM.date.substring(0, 7) : false)
      : prevM !== null && periodKey(m) !== periodKey(prevM)
    if (changed) monthBoundaries.push(slots[i - a])
    prevM = m
  }

  // подписи оси: каждые candleStep свечей по абсолютному индексу — не прыгают при сдвиге окна
  const targetDistance = 80
  const candleStep = Math.max(1, Math.round(targetDistance / Math.max(1, step)))
  const ticks = []
  for (let i = Math.ceil(a / candleStep) * candleStep; i < b; i += candleStep) ticks.push(slots[i - a])
  const count = totalDays.value
  if (b === count && count > 0) {
    const lastSlot = slots[slots.length - 1]
    if (ticks.length > 0 && ticks[ticks.length - 1].key !== lastSlot.key) {
      if (lastSlot.x - ticks[ticks.length - 1].x < targetDistance * 0.6) ticks[ticks.length - 1] = lastSlot
      else ticks.push(lastSlot)
    }
  }

  return { candles, ticks, monthBoundaries, markers, step, count }
})

// Выбор свечи: клик по ней (не перетаскивание); повторный клик снимает. Наружу — границы корзины и вклад Binance.
const selectedIdx = computed(() => {
  const S = series.value
  if (!S || selectedKey.value === null) return -1
  const i = lowerBound(S.starts, Number(selectedKey.value))
  return i < S.n && S.starts[i] === Number(selectedKey.value) ? i : -1
})
const selectCandle = (c) => {
  if (!c || c.key === selectedKey.value) {
    selectedKey.value = null
    emit('select', null)
    return
  }
  selectedKey.value = c.key
  const S = series.value
  emit('select', {
    key: c.key, title: hoverTitle(c),
    from: c.ts, to: c.i + 1 < S.n ? S.starts[c.i + 1] : Infinity, // последняя корзина забирает и «будущие» операции
    open: c.open, close: c.close, high: c.high, low: c.low, bnOpen: c.bnOpen, bnClose: c.bnClose, bnSeries: c.bnSeries
  })
}
defineExpose({ clearSelection: () => selectCandle(null) })
const candleAtX = (x) => {
  const S = series.value
  if (!S) return null
  const i = Math.round((x - paddingX - stepPx.value / 2) / stepPx.value)
  return i >= 0 && i < S.n ? candleAt(i) : null
}

const tickLabel = (t) => (isDaily.value || t.future ? formatShortDateName(t.date) : t.time)
const boundaryLabel = (m) => (isDaily.value || m.future ? formatMonthName(m.date) : formatShortDateName(m.date))
const hoverTitle = (c) => {
  if (tfId.value === '1W') return `Неделя с ${formatDateLong(c.date)}`
  return isDaily.value ? formatDateLong(c.date) : `${formatDateLong(c.date)} · ${c.time}`
}

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
  if (Math.abs(e.clientX - dragStartX) < 4) { // это клик, а не перетаскивание
    const container = chartContainerRef.value
    const r = container.getBoundingClientRect()
    if (props.selectable) selectCandle(candleAtX(e.clientX - r.left + container.scrollLeft))
    return
  }
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

// ── Подгрузка истории чанками, как на биржах: подошли к левому краю — тянем ещё окно, пока идёт запрос график живёт.
// Запрос в бэкенд — только за новыми корзинами (старые лежат в кеше); окно расширяется после ответа и позиция сохраняется.
const loadingMore = ref(false)
const loadMore = async () => {
  if (loadingMore.value || !canLoadMore.value) return
  const el = chartContainerRef.value
  if (!el) return
  const def = tfDef.value
  const tf = tfId.value
  const epoch = bnEpoch
  loadingMore.value = true
  try {
    const bigger = startsFor(extraBack.value + def.limit)
    const added = bigger.length - bucketStarts.value.length
    if (added <= 0) return
    if (bnHas.value) {
      const cache = bnCache()
      const need = bigger.slice(0, added).filter(s => !cache.has(s))
      if (need.length && !(await fetchBnRun(bigger, 0, added - 1, epoch, tf))) return
    }
    if (tf !== tfId.value || epoch !== bnEpoch) return // за время запроса сменили таймфрейм или данные
    const before = el.scrollLeft
    extraBack.value += def.limit
    bnVer.value++
    await nextTick()
    el.scrollLeft = before + added * stepPx.value // новые свечи слева: остаёмся на тех же
  } catch { /* связь пропала — попробуем при следующем сдвиге */ } finally {
    loadingMore.value = false
  }
}

let scrollRaf = 0
const syncView = () => {
  scrollRaf = 0
  const el = chartContainerRef.value
  if (!el) return
  scrollX.value = el.scrollLeft
  viewW.value = el.clientWidth
  if (el.scrollLeft < el.clientWidth * 1.5) loadMore()
}
const onScroll = () => { if (!scrollRaf) scrollRaf = requestAnimationFrame(syncView) }
onBeforeUnmount(() => cancelAnimationFrame(scrollRaf))
onMounted(syncView)

const resetZoom = () => {
  zoomLevel.value = 1.0
  scrollToEnd(true)
}

// «Свежий край» — сегодняшний день (справа от него может лежать таймлайн вперёд)
const scrollToEnd = (force = false) => {
  if (!force && hasInitialScrolled.value) return

  nextTick(() => {
    const el = chartContainerRef.value
    if (!el) return
    const S = series.value
    el.scrollLeft = futureDays.value.length && S ? Math.max(0, slotX(S.n - 1) - el.clientWidth * 0.7) : el.scrollWidth
    hasInitialScrolled.value = true
    syncView()
  })
}

onMounted(() => {
  scrollToEnd()
})

// Вкладки живут в KeepAlive: у скрытого блока браузер сбрасывает прокрутку в 0 без события scroll, а окно свечей
// считается по запомненному scrollX — график оставался пустым. При уходе запоминаем позицию, при возврате ставим её и пересчитываем окно.
let savedScroll = null
onDeactivated(() => { savedScroll = scrollX.value }) // к этому моменту блок уже скрыт и его scrollLeft обнулён — берём последний синхронизированный
onActivated(() => {
  if (savedScroll === null) return // первая активация — это обычный mount
  const want = savedScroll
  nextTick(() => {
    const el = chartContainerRef.value
    if (!el) return
    el.scrollLeft = want
    syncView()
  })
})

// Данные приходят после монтирования: когда свечи впервые появились — встаём на свежий край
let scrolledWithData = false
// Выплаты приходят позже свечей и растягивают ось вперёд — встаём на сегодня ещё раз (один раз)
let scrolledWithFuture = false
watch(() => futureDays.value.length, (n) => {
  if (n > 0 && !scrolledWithFuture) {
    scrolledWithFuture = true
    scrollToEnd(true)
  }
})
watch(() => (series.value ? series.value.n : 0), (n) => {
  if (n > 0 && !scrolledWithData) {
    scrolledWithData = true
    scrollToEnd(true)
  }
})
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 p-5 rounded space-y-4">
    <div class="flex items-center justify-between text-xs border-b border-slate-800/80 pb-3">
      <div class="flex items-center gap-3 shrink-0 whitespace-nowrap">
        <span class="font-bold text-slate-300 uppercase tracking-wider">Динамика капитала</span>
        <div class="flex items-center gap-0.5">
          <button
            v-for="t in TFS"
            :key="t.id"
            type="button"
            class="text-[10px] font-bold px-1.5 py-0.5 rounded border transition-colors"
            :class="t.id === tfId ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'text-slate-500 border-transparent hover:text-slate-300'"
            @click="setTf(t.id)"
          >{{ t.id }}</button>
        </div>
        <span class="text-[10px] text-slate-500">Свечей: {{ series ? series.n : 0 }}<template v-if="loadingMore"> · грузим…</template></span>
        <button 
          @click="resetZoom" 
          class="text-[10px] text-slate-400 hover:text-emerald-400 underline transition-colors"
        >
          Сброс ({{ Math.round(zoomLevel * 100) }}%)
        </button>
      </div>

      <div v-if="hoveredMarker" class="flex items-center gap-4 text-[11px] font-mono min-w-0 overflow-hidden whitespace-nowrap">
        <span class="text-slate-200 font-bold">{{ formatDateLong(hoveredMarker.date) }}</span>
        <span :style="{ color: hoveredMarker.color || '#fbbf24' }">{{ hoveredMarker.kind === 'coupon' ? 'Купон' : 'Погашение' }}</span>
        <span class="text-slate-400">{{ hoveredMarker.isin }}</span>
        <strong class="text-emerald-400">+₴ {{ formatMoney(hoveredMarker.amount) }}</strong>
        <span class="text-slate-500">за 1 шт. ({{ hoveredMarker.unit }} из {{ hoveredMarker.of }})</span>
        <span class="text-slate-500">{{ hoveredMarker.done ? 'получено' : 'по графику' }}</span>
      </div>
      <div v-else-if="hoveredCandle" class="flex items-center gap-4 text-[11px] font-mono min-w-0 overflow-hidden whitespace-nowrap">
        <span class="text-slate-200 font-bold">{{ hoverTitle(hoveredCandle) }}</span>
        <span class="text-slate-500">Откр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.open) }}</strong></span>
        <span class="text-slate-500">Пик: <strong class="text-emerald-400">{{ formatMoney(hoveredCandle.high) }}</strong></span>
        <span class="text-slate-500">Мин: <strong class="text-rose-400">{{ formatMoney(hoveredCandle.low) }}</strong></span>
        <span class="text-slate-500">Закр: <strong class="text-slate-200">{{ formatMoney(hoveredCandle.close) }}</strong></span>
      </div>
      <div v-else class="text-[10px] text-slate-600 uppercase">Колесико — Zoom, Перетаскивание — Навигация</div>
    </div>

    <div class="flex">
    <div
      ref="chartContainerRef"
      data-wheel="own"
      @wheel.prevent="handleWheel"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @scroll.passive="onScroll"
      class="chart-pan min-w-0 flex-1 overflow-x-auto"
      :class="isDragging ? 'cursor-grabbing' : 'cursor-grab'"
    >
      <svg
        v-if="series"
        :width="svgWidth"
        :height="svgHeight"
        class="select-none block"
      >
        <line v-for="p in priceTicks" :key="'h-grid-' + p.val" x1="0" :y1="p.y" :x2="svgWidth" :y2="p.y" stroke="#1e293b" stroke-dasharray="3 3" />
        <line v-if="lastPrice" x1="0" :y1="lastPrice.y" :x2="svgWidth" :y2="lastPrice.y" :stroke="lastPrice.up ? '#10b981' : '#f43f5e'" stroke-opacity="0.55" stroke-dasharray="1 3" />

        <line
          v-for="t in candleScales.ticks"
          :key="'v-grid-' + t.key"
          :x1="t.x"
          y1="10"
          :x2="t.x"
          :y2="svgHeight - 25"
          stroke="#1e293b"
          stroke-dasharray="2 2"
        />

        <g v-for="m in candleScales.monthBoundaries" :key="'m-boundary-' + m.key">
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
            {{ boundaryLabel(m) }}
          </text>
        </g>

        <!-- сегодня: справа начинается таймлайн вперёд -->
        <g v-if="futureDays.length && series">
          <line :x1="slotX(series.n - 1) + candleScales.step / 2" y1="10" :x2="slotX(series.n - 1) + candleScales.step / 2" :y2="svgHeight - 25" stroke="#38bdf8" stroke-opacity="0.35" stroke-dasharray="1 3" />
        </g>

        <rect
          v-if="selectedIdx >= 0"
          :x="slotX(selectedIdx) - candleScales.step / 2"
          y="8"
          :width="candleScales.step"
          :height="svgHeight - 30"
          fill="#38bdf8"
          fill-opacity="0.12"
          stroke="#38bdf8"
          stroke-opacity="0.5"
        />

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
          :key="c.key"
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
            :class="hoveredCandle && hoveredCandle.key === c.key ? 'opacity-100 stroke-2' : 'opacity-90'"
          />
          <rect
            :x="c.x - c.width / 2"
            :y="c.bodyTop"
            :width="c.width"
            :height="c.bodyHeight"
            :fill="c.isBullish ? '#10b981' : '#f43f5e'"
            rx="1"
            class="transition-opacity"
            :class="hoveredCandle && hoveredCandle.key === c.key ? 'opacity-100' : 'opacity-90'"
          />
        </g>

        <!-- выплаты: купон — листочек с треугольником на грани, погашение — «бумага»; серые — уже получены -->
        <g
          v-for="mk in candleScales.markers"
          :key="mk.key"
          :transform="`translate(${mk.x} ${mk.y})`"
          @mouseenter="!isDragging && !mk.more && (hoveredMarker = mk)"
          @mouseleave="hoveredMarker = null"
        >
          <text v-if="mk.more" x="0" y="0" fill-opacity="0.95" :fill="mk.done ? CLAIMED : (mk.color || BLUE)" font-size="9" font-weight="bold" font-family="monospace">x{{ mk.n }}</text>
          <g v-else-if="mk.kind === 'coupon'" transform="scale(1.25)" :stroke="mk.done ? CLAIMED : (mk.color || BLUE)" :fill="mk.done ? CLAIMED : (mk.color || BLUE)" stroke-width="1" stroke-linejoin="round" stroke-linecap="round">
            <path d="M-4.5 4.5 C-6 -2 -1 -5.5 5 -5.5 C5 0 2 5.5 -4.5 4.5 Z" fill-opacity="0.3" />
            <path d="M-4.5 4.5 L3 -3" fill="none" />
            <path d="M-4.5 4.5 L-6.5 6.5" fill="none" />
            <path d="M5 -5.5 L8 -8 L8 -3 Z" />
          </g>
          <g v-else transform="scale(1.25)" :stroke="mk.done ? CLAIMED : (mk.color || BLUE)" fill="none" stroke-width="1.2" stroke-linejoin="round" stroke-linecap="round">
            <path d="M-4 -6 H1.5 L4.5 -3 V6 H-4 Z" fill="#0f172a" />
            <path d="M1.5 -6 V-3 H4.5" />
            <path d="M-2 0 H2.2 M-2 2.5 H2.2" />
          </g>
        </g>

        <g>
          <text
            v-for="t in candleScales.ticks"
            :key="'tick-text-' + t.key"
            :x="t.x"
            :y="svgHeight - 6"
            text-anchor="middle"
            fill="#64748b"
            font-size="10"
            font-family="monospace"
          >
            {{ tickLabel(t) }}
          </text>
        </g>
      </svg>

      <div v-else class="h-48 flex items-center justify-center text-xs text-slate-600">
        Нет транзакций для построения свечей. Нажмите SYNC.
      </div>
    </div>
    <!-- шкала цены: стоит на месте, пока свечи листаются -->
    <svg v-if="series" width="58" :height="svgHeight" class="select-none block shrink-0 border-l border-slate-800">
      <text v-for="p in priceTicks" :key="'p-' + p.val" x="6" :y="p.y + 3" fill="#64748b" font-size="10" font-family="monospace">{{ p.label }}</text>
      <g v-if="lastPrice">
        <rect x="1" :y="lastPrice.y - 8" width="56" height="16" rx="2" :fill="lastPrice.up ? '#10b981' : '#f43f5e'" />
        <text x="5" :y="lastPrice.y + 3.5" fill="#020617" font-size="9" font-weight="bold" font-family="monospace">₴ {{ lastLabel }}</text>
      </g>
    </svg>
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

<script setup>
import { computed, ref, watch } from 'vue'
import { formatMoney } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import { apiGetJson, q } from '../utils/api.js'
import { bankTheme } from '../utils/bankTheme.js'
import { prepareAccount } from '../utils/accountView.js'
import AccountPanel from '../components/AccountPanel.vue'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

// Выплаты по облигациям брокерских счетов (купоны и погашения) — маркеры на графике и таймлайн вперёд.
// Счета обновляются вместе с версией данных, поэтому перечитываем при каждой новой загрузке.
const allPayEvents = ref([])
watch(() => props.accounts, async (list) => {
  const manual = (list || []).filter(a => a.bank_key === 'icu')
  try {
    const all = await Promise.all(manual.map(async a => {
      const ov = await apiGetJson(`/icu/ledger?account_id=${q(a.id)}`)
      const color = bankTheme(a.bank_key || 'icu', a.type).color // маркеры в цвет счёта
      return (ov.events || []).map(e => ({ ...e, color, account_id: a.id }))
    }))
    allPayEvents.value = all.flat()
  } catch (e) {
    console.error('ICU events error:', e)
  }
}, { immediate: true })

const now = new Date()
const currentYear = now.getFullYear()
const currentMonth = now.getMonth()

const uahAccounts = computed(() => props.accounts.filter(a => a.currency_code === 980))

// ВРЕМЕННО (отладка графика): слева все счета как в «Операционной», на графике — только выбранные. null — выбраны все,
// пока ничего не трогали (новые счета во время разведки подхватываются сами).
const preparedAccounts = computed(() => props.accounts.map(prepareAccount))
const chosen = ref(null)
const isChosen = (id) => chosen.value === null || chosen.value.has(id)
const toggleAcc = (id) => {
  const next = new Set(chosen.value === null ? props.accounts.map(a => a.id) : chosen.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  chosen.value = next
}
const chooseAll = () => { chosen.value = null }
const chooseNone = () => { chosen.value = new Set() }
const isUah = (acc) => acc.currency_code === 980
const chartAccounts = computed(() => uahAccounts.value.filter(a => isChosen(a.id)))

// Капитал на карточке — по всем гривневым счетам; на графике — по выбранным (валютные в ₴-график не входят и раньше)
const totalValueUAH = computed(() => uahAccounts.value.reduce((sum, a) => sum + a.balance, 0))
const chartTotalUAH = computed(() => chartAccounts.value.reduce((sum, a) => sum + a.balance, 0))

// Капитал считается по гривневым счетам, значит и динамика/месячные итоги — только по их транзакциям
// (иначе валютные операции искажают свечи относительно текущего баланса)
const uahTransactions = computed(() => {
  const ids = new Set(uahAccounts.value.map(a => a.id))
  return props.transactions.filter(t => ids.has(t.account_id))
})
const chartIds = computed(() => new Set(chartAccounts.value.map(a => a.id)))
const chartTransactions = computed(() => uahTransactions.value.filter(t => chartIds.value.has(t.account_id)))
// Брокерский счёт (ICU): маркеры купонов/погашений и таймлайн вперёд — только по выбранным счетам
const payEvents = computed(() => allPayEvents.value.filter(e => chartIds.value.has(e.account_id)))

// Операции Binance — обычная история счёта: свечи двигаются по ним сразу, пока разведка идёт. Когда собран минутный ряд
// оценки, скачки депозитов уже в нём, и график сам перестаёт считать эти операции второй раз (CandleChart, binanceIds).
const binanceIds = computed(() => new Set(props.accounts.filter(a => a.bank_key === 'binance').map(a => a.id)))
// крипто-счёт влияет на график (минутный ряд оценки), только если он выбран
const binanceIdList = computed(() => [...binanceIds.value].filter(id => chartIds.value.has(id)))

const currentMonthTransactions = computed(() => {
  return uahTransactions.value.filter(t => {
    if (!t.time) return false
    const d = new Date(t.time)
    return d.getFullYear() === currentYear && d.getMonth() === currentMonth
  })
})

const monthlyInflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount > 0 && !t.internal)
    .reduce((sum, t) => sum + t.amount, 0)
})

const monthlyOutflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount < 0 && !t.internal)
    .reduce((sum, t) => sum + Math.abs(t.amount), 0)
})

const monthlyDelta = computed(() => {
  return monthlyInflow.value - monthlyOutflow.value
})

// ── Разбор выбранной свечи (клик по ней): операции корзины, разделённые по счетам/картам. Нужен для отладки графика:
// сумма операций корзины + вклад Binance должны сходиться с (закрытие − открытие).
const picked = ref(null)
watch(chosen, () => { picked.value = null })
const accById = computed(() => new Map(props.accounts.map(a => [a.id, a])))
const accTitle = (id) => {
  const a = accById.value.get(id)
  if (!a) return id
  const tail = (a.pans && a.pans[0]) || a.masked_pan || a.iban || ''
  return tail && a.bank_key !== 'binance' ? `${a.name} · ${String(tail).slice(-8)}` : a.name
}
const pickedGroups = computed(() => {
  const p = picked.value
  if (!p) return []
  const by = new Map()
  for (const t of chartTransactions.value) {
    if (!t.time) continue
    const ms = new Date(t.time).getTime()
    if (ms < p.from || ms >= p.to) continue
    if (!by.has(t.account_id)) by.set(t.account_id, { id: t.account_id, title: accTitle(t.account_id), inflow: 0, outflow: 0, rows: [] })
    const g = by.get(t.account_id)
    if (t.amount > 0) g.inflow += t.amount
    else g.outflow += -t.amount
    g.rows.push({ ...t, ms })
  }
  const list = [...by.values()]
  list.forEach(g => g.rows.sort((a, b) => a.ms - b.ms))
  return list.sort((a, b) => a.title.localeCompare(b.title))
})
const isBn = (g) => binanceIds.value.has(g.id)
const inValuation = (g) => isBn(g) && !!picked.value?.bnSeries // операции Binance «в оценке» только при минутном ряде
const pickedNet = computed(() => pickedGroups.value.filter(g => !inValuation(g)).reduce((s, g) => s + g.inflow - g.outflow, 0)) // без Binance, если он учтён Δ оценки
const pickedBn = computed(() => (picked.value ? picked.value.bnClose - picked.value.bnOpen : 0))
const fmtTime = (ms) => new Date(ms).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
const signed = (v) => (v > 0 ? '+' : '') + formatMoney(v)
</script>

<template>
  <main class="space-y-6">
    <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-slate-900/60 border border-slate-800 p-5 rounded space-y-2">
        <span class="text-[10px] text-slate-500 uppercase tracking-wider block">Капитал</span>
        <div class="flex flex-wrap items-baseline gap-3">
          <span class="text-2xl font-bold text-white">{{ formatMoney(totalValueUAH) }} ₴</span>
          <span
            class="text-xs font-bold px-2 py-0.5 rounded border"
            :class="monthlyDelta >= 0 ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
          >
            {{ monthlyDelta >= 0 ? '+' : '' }}{{ formatMoney(monthlyDelta) }} ₴
          </span>
        </div>
      </div>
    </section>

    <div class="grid grid-cols-1 lg:grid-cols-[17rem_minmax(0,1fr)] gap-6 items-start">
    <!-- ВРЕМЕННО: выбор счетов для графика -->
    <aside class="space-y-3 font-mono select-none lg:sticky lg:top-4">
      <div class="flex items-center justify-between text-[10px] uppercase tracking-wider text-slate-500">
        <span>Счета на графике</span>
        <span class="flex gap-3">
          <button class="hover:text-slate-200 underline" @click="chooseAll">все</button>
          <button class="hover:text-slate-200 underline" @click="chooseNone">никакие</button>
        </span>
      </div>
      <div class="space-y-2 max-h-[70vh] overflow-y-auto pr-1">
        <div v-for="acc in preparedAccounts" :key="acc.key" :class="!isUah(acc) && 'opacity-40'" :title="isUah(acc) ? '' : 'Не в гривне — в ₴-график не входит'">
          <AccountPanel
            :card="acc"
            compact
            selectable
            :selected="isUah(acc) && isChosen(acc.id)"
            role="checkbox"
            tabindex="0"
            :aria-checked="isUah(acc) && isChosen(acc.id)"
            @click="isUah(acc) && toggleAcc(acc.id)"
            @keydown.enter.prevent="isUah(acc) && toggleAcc(acc.id)"
            @keydown.space.prevent="isUah(acc) && toggleAcc(acc.id)"
          />
        </div>
      </div>
    </aside>
    <div class="space-y-6 min-w-0">
    <CandleChart :transactions="chartTransactions" :binance-ids="binanceIdList" :total-value-u-a-h="chartTotalUAH" :events="payEvents" @select="picked = $event" />

    <!-- разбор выбранной свечи: операции по картам -->
    <section v-if="picked" class="bg-slate-900/60 border border-slate-800 rounded font-mono text-xs">
      <div class="flex flex-wrap items-baseline justify-between gap-3 p-3 border-b border-slate-800">
        <div class="text-slate-200 font-bold">{{ picked.title }}</div>
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-slate-500">
          <span>Откр: <strong class="text-slate-200">{{ formatMoney(picked.open) }}</strong></span>
          <span>Закр: <strong class="text-slate-200">{{ formatMoney(picked.close) }}</strong></span>
          <span>Δ свечи: <strong :class="picked.close - picked.open >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signed(picked.close - picked.open) }}</strong></span>
          <span>операции карт: <strong class="text-slate-200">{{ signed(pickedNet) }}</strong></span>
          <span v-if="pickedBn">Δ оценки Binance: <strong class="text-yellow-300">{{ signed(pickedBn) }}</strong></span>
        </div>
      </div>
      <div v-if="!pickedGroups.length" class="p-3 text-slate-500">В этой свече операций нет<template v-if="pickedBn"> — только изменение оценки Binance</template>.</div>
      <div v-for="g in pickedGroups" :key="g.id" class="border-b border-slate-800/60 last:border-b-0">
        <div class="flex items-center justify-between gap-3 px-3 py-2 bg-slate-950/40">
          <span class="text-slate-300 font-bold truncate">{{ g.title }}<span v-if="inValuation(g)" class="ml-2 text-[9px] font-normal uppercase text-yellow-400">внешние движения, в оценке</span></span>
          <span class="shrink-0 text-[11px]">
            <span class="text-emerald-400">+{{ formatMoney(g.inflow) }}</span>
            <span class="text-slate-600"> / </span>
            <span class="text-rose-400">−{{ formatMoney(g.outflow) }}</span>
            <span class="text-slate-500"> = </span>
            <strong :class="g.inflow - g.outflow >= 0 ? 'text-emerald-300' : 'text-rose-300'">{{ signed(g.inflow - g.outflow) }}</strong>
          </span>
        </div>
        <div v-for="t in g.rows" :key="t.id" class="flex items-baseline gap-3 px-3 py-1 border-t border-slate-800/40">
          <span class="w-28 shrink-0 text-slate-500">{{ fmtTime(t.ms) }}</span>
          <span class="min-w-0 flex-1 truncate text-slate-300" :title="t.description">{{ t.description }}<span v-if="t.internal" class="ml-2 text-[9px] uppercase text-sky-400">внутр.</span></span>
          <span class="shrink-0 font-bold" :class="t.amount >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signed(t.amount) }}</span>
        </div>
      </div>
    </section>
    </div>
    </div>
  </main>
</template>
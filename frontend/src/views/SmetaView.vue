<script setup>
import { computed, toRef, onMounted } from 'vue'
import { money } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import SyncButton from '../components/SyncButton.vue'
import CapitalStructure from '../components/CapitalStructure.vue'
import BondPortfolio from '../components/BondPortfolio.vue'
import { useMacro } from '../composables/useMacro.js'
import { FX_IDS } from '../utils/macro.js'
import { fxRates, dollarAssumptions } from '../utils/dollar.js'
import { capitalStructure } from '../utils/capital.js'
import { toLocalDateStr } from '../utils/formatters.js'
import { useCapitalChart } from '../composables/useCapitalChart.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

const now = new Date()
const currentYear = now.getFullYear()
const currentMonth = now.getMonth()

// График Сметы — всё время по всем счетам (выбор карт и свечей живёт в «Операционной»)
const { uahAccounts, uahTransactions, chartTransactions, chartTotalUAH, binanceIdList, payEvents, positions } =
  useCapitalChart(toRef(props, 'accounts'), toRef(props, 'transactions'))

// Инфляция для реальной доходности портфеля (ряды НБУ из кеша)
const macro = useMacro()
// Курсы НБУ (доллар, евро) — капитал в долларовом эквиваленте; без них всё остаётся в гривне
const fx = useMacro([...FX_IDS, 'us_cpi_yoy'])
onMounted(() => { macro.load(); fx.load() })
const rates = computed(() => { const r = fxRates(fx.series.value); return r.usdUah ? r : null })
const assume = computed(() => dollarAssumptions(fx.series.value, toLocalDateStr(new Date())))
const today = computed(() => toLocalDateStr(new Date()))

const totalValueUAH = computed(() => uahAccounts.value.reduce((sum, a) => sum + a.balance, 0))
// Капитал: с курсом — в долларах (валютные счета как есть, гривна и Binance по курсу), рядом приблизительный гривневый эквивалент
const cap = computed(() => capitalStructure(props.accounts, rates.value))
const unit = computed(() => (rates.value ? '$' : '₴'))

const currentMonthTransactions = computed(() => {
  return uahTransactions.value.filter(t => {
    if (!t.time) return false
    const d = new Date(t.time)
    return d.getFullYear() === currentYear && d.getMonth() === currentMonth
  })
})

const monthlyInflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount > 0 && !t.internal && !t.own_account_id)
    .reduce((sum, t) => sum + t.amount, 0)
})

const monthlyOutflow = computed(() => {
  return currentMonthTransactions.value
    .filter(t => t.amount < 0 && !t.internal && !t.own_account_id)
    .reduce((sum, t) => sum + Math.abs(t.amount), 0)
})

const monthName = new Date().toLocaleDateString('ru-RU', { month: 'long' })
// Месячные суммы в той же единице, что и капитал (гривна переводится по курсу НБУ)
const inUnit = (uah) => (rates.value ? money(uah / rates.value.usdUah, '$') : money(uah))

const monthlyDelta = computed(() => {
  return monthlyInflow.value - monthlyOutflow.value
})
</script>

<template>
  <main class="space-y-5">
    <section class="bg-slate-900/60 border border-slate-800 rounded p-5 flex flex-wrap items-start gap-x-12 gap-y-5">
      <div class="space-y-1">
        <span class="text-[10px] text-slate-500 uppercase tracking-wider block">Капитал</span>
        <div class="text-3xl font-bold text-white tabular-nums">{{ rates ? money(cap.total, '$') : money(totalValueUAH) }}</div>
        <div v-if="rates" class="text-xs text-slate-500 tabular-nums" :title="`Приблизительно, по курсу НБУ ${rates.usdUah} ₴ за доллар`">≈ {{ money(cap.totalUah) }}</div>
      </div>
      <div class="space-y-1">
        <span class="text-[10px] text-slate-500 uppercase tracking-wider block">За {{ monthName }}</span>
        <div class="flex flex-wrap gap-x-8 gap-y-2">
          <div title="Приход за месяц: без переводов между своими счетами"><div class="text-[11px] text-slate-500">Приход</div><div class="text-lg font-bold tabular-nums text-emerald-400">{{ inUnit(monthlyInflow) }}</div></div>
          <div title="Расход за месяц: без переводов между своими счетами"><div class="text-[11px] text-slate-500">Расход</div><div class="text-lg font-bold tabular-nums text-rose-400">{{ inUnit(monthlyOutflow) }}</div></div>
          <div title="Приход минус расход"><div class="text-[11px] text-slate-500">Итог</div><div class="text-lg font-bold tabular-nums" :class="monthlyDelta >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ monthlyDelta >= 0 ? '+' : '−' }}{{ inUnit(Math.abs(monthlyDelta)) }}</div></div>
        </div>
      </div>
      <SyncButton class="ml-auto" />
    </section>

    <div class="grid gap-4 lg:grid-cols-2 items-stretch">
      <CapitalStructure :accounts="accounts" :rates="rates" />
      <BondPortfolio :positions="positions" :macro="macro.series.value" :today="today" :assume="assume" />
    </div>
    <CandleChart :transactions="chartTransactions" :binance-ids="binanceIdList" :total-value-u-a-h="chartTotalUAH" :events="payEvents" />
  </main>
</template>

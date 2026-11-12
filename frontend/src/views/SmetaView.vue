<script setup>
import { computed, toRef, onMounted } from 'vue'
import { money, signed } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import SyncButton from '../components/SyncButton.vue'
import CapitalStructure from '../components/CapitalStructure.vue'
import BondPortfolio from '../components/BondPortfolio.vue'
import { useMacro } from '../composables/useMacro.js'
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
onMounted(() => macro.load())
const today = computed(() => toLocalDateStr(new Date()))

const totalValueUAH = computed(() => uahAccounts.value.reduce((sum, a) => sum + a.balance, 0))

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

const monthlyDelta = computed(() => {
  return monthlyInflow.value - monthlyOutflow.value
})
</script>

<template>
  <main class="space-y-6">
    <section class="flex flex-wrap items-start justify-between gap-4">
      <div class="bg-slate-900/60 border border-slate-800 p-5 rounded space-y-2 w-full sm:w-64">
        <span class="text-[10px] text-slate-500 uppercase tracking-wider block">Капитал</span>
        <div class="text-2xl font-bold text-white">{{ money(totalValueUAH) }}<sub class="font-bold ml-1" style="font-size:11px;vertical-align:baseline;position:relative;top:0.35em" :class="monthlyDelta >= 0 ? 'text-emerald-400' : 'text-rose-400'" title="За месяц">{{ signed(monthlyDelta) }}</sub></div>
      </div>
      <SyncButton />
    </section>

    <div class="grid gap-4 md:grid-cols-2">
      <CapitalStructure :accounts="accounts" />
      <BondPortfolio :positions="positions" :macro="macro.series.value" :today="today" />
    </div>

    <CandleChart :transactions="chartTransactions" :binance-ids="binanceIdList" :total-value-u-a-h="chartTotalUAH" :events="payEvents" />
  </main>
</template>

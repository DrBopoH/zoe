<script setup>
import { computed, toRef } from 'vue'
import { formatMoney } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import { useCapitalChart } from '../composables/useCapitalChart.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

const now = new Date()
const currentYear = now.getFullYear()
const currentMonth = now.getMonth()

// График Сметы — всё время по всем счетам (выбор карт и свечей живёт в «Операционной»)
const { uahAccounts, uahTransactions, chartTransactions, chartTotalUAH, binanceIdList, payEvents } =
  useCapitalChart(toRef(props, 'accounts'), toRef(props, 'transactions'))

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

    <CandleChart :transactions="chartTransactions" :binance-ids="binanceIdList" :total-value-u-a-h="chartTotalUAH" :events="payEvents" />
  </main>
</template>

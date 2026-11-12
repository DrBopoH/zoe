<script setup>
import { computed, ref, watch } from 'vue'
import { formatMoney } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import { apiGetJson, q } from '../utils/api.js'
import { bankTheme } from '../utils/bankTheme.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

// Выплаты по облигациям инвестиционных счетов (купоны и погашения) — маркеры на графике и таймлайн вперёд.
// Счета обновляются вместе с версией данных, поэтому перечитываем при каждой новой загрузке.
const payEvents = ref([])
watch(() => props.accounts, async (list) => {
  const manual = (list || []).filter(a => a.manual)
  try {
    const all = await Promise.all(manual.map(async a => {
      const ov = await apiGetJson(`/icu/ledger?account_id=${q(a.id)}`)
      const color = bankTheme(a.bank_key || 'icu', a.type).color // маркеры в цвет счёта
      return (ov.events || []).map(e => ({ ...e, color }))
    }))
    payEvents.value = all.flat()
  } catch (e) {
    console.error('ICU events error:', e)
  }
}, { immediate: true })

const now = new Date()
const currentYear = now.getFullYear()
const currentMonth = now.getMonth()

const uahAccounts = computed(() => props.accounts.filter(a => a.currency_code === 980))

const totalValueUAH = computed(() => {
  return uahAccounts.value.reduce((sum, a) => sum + a.balance, 0)
})

// Капитал считается по гривневым счетам, значит и динамика/месячные итоги — только по их транзакциям
// (иначе валютные операции искажают свечи относительно текущего баланса)
const uahTransactions = computed(() => {
  const ids = new Set(uahAccounts.value.map(a => a.id))
  return props.transactions.filter(t => ids.has(t.account_id))
})

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

    <CandleChart :transactions="uahTransactions" :total-value-u-a-h="totalValueUAH" :events="payEvents" />
  </main>
</template>
<script setup>
import { computed, toRef, onMounted } from 'vue'
import { money, moneySigned } from '../utils/formatters.js'
import CandleChart from '../components/CandleChart.vue'
import { useMacro } from '../composables/useMacro.js'
import { fxRates } from '../utils/dollar.js'
import { capitalStructure } from '../utils/capital.js'
import { useCapitalChart } from '../composables/useCapitalChart.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

const now = new Date()

// Главная — пустая страница до переборки дизайна в Build 3: только первые элементы старой Сметы (плитка капитала и график динамики) на новых компонентах. График — всё время по всем счетам (выбор карт и свечей живёт в «Операционной»)
const { uahAccounts, uahTransactions, chartTransactions, chartTotalUAH, binanceIdList, payEvents } =
  useCapitalChart(toRef(props, 'accounts'), toRef(props, 'transactions'))

// Курсы НБУ — капитал в долларовом эквиваленте; без них всё остаётся в гривне
const fx = useMacro('fx', ['us_cpi_yoy'])
onMounted(() => fx.load())
const rates = computed(() => { const r = fxRates(fx.series.value); return r.usdUah ? r : null })

const totalValueUAH = computed(() => uahAccounts.value.reduce((sum, a) => sum + a.balance, 0))
const cap = computed(() => capitalStructure(props.accounts, rates.value))

// Итог за месяц: приход минус расход без переводов между своими счетами, в той же единице, что и капитал
const monthlyDeltaUAH = computed(() =>
  uahTransactions.value
    .filter((t) => {
      if (!t.time || t.internal || t.own_account_id) return false
      const d = new Date(t.time)
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
    })
    .reduce((sum, t) => sum + t.amount, 0)
)
const monthName = now.toLocaleDateString('ru-RU', { month: 'long' })
const sym = computed(() => (rates.value ? '$' : '₴'))
const monthly = computed(() => (rates.value ? monthlyDeltaUAH.value / rates.value.usdUah : monthlyDeltaUAH.value))
</script>

<template>
  <main class="space-y-5">
    <section class="inline-block min-w-[17rem] bg-slate-900/60 border border-slate-800 rounded-lg p-5 space-y-2">
      <span class="text-[10px] text-slate-500 uppercase tracking-wider block">Капитал</span>
      <div class="text-3xl font-bold text-white tabular-nums">{{ rates ? money(cap.total, '$') : money(totalValueUAH) }}</div>
      <div v-if="rates" class="text-xs text-slate-500 tabular-nums" :title="`Приблизительно, по курсу НБУ ${rates.usdUah} ₴ за доллар`">≈ {{ money(cap.totalUah) }}</div>
      <div
        class="inline-flex items-baseline gap-1.5 rounded border px-2 py-0.5 text-xs font-bold tabular-nums"
        :class="monthly >= 0 ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-400'"
        title="Приход минус расход за месяц: без переводов между своими счетами"
      >
        {{ moneySigned(monthly, sym) }}
        <span class="text-[9px] font-normal uppercase opacity-70">за {{ monthName }}</span>
      </div>
    </section>

    <CandleChart :transactions="chartTransactions" :binance-ids="binanceIdList" :total-value-u-a-h="chartTotalUAH" :events="payEvents" />
  </main>
</template>

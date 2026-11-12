<script setup>
import { computed, ref, toRef } from 'vue'
import { prepareAccount } from '../utils/accountView.js'
import AccountPanel from '../components/AccountPanel.vue'
import AccountHistoryTable from '../components/AccountHistoryTable.vue'
import CandleChart from '../components/CandleChart.vue'
import { useCapitalChart } from '../composables/useCapitalChart.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] }
})

const preparedAccounts = computed(() => props.accounts.map(prepareAccount))

// Мультиселект плиток (как в приложении ПриватБанка): клик добавляет/снимает карту, история — по выбранным
const selectedIds = ref([])
const isSelected = (acc) => selectedIds.value.includes(acc.key)
const toggle = (acc) => {
  selectedIds.value = isSelected(acc)
    ? selectedIds.value.filter(id => id !== acc.key)
    : [...selectedIds.value, acc.key]
}

const toRow = (a) => ({
  id: a.id,
  label: a.pans[0] || a.displayIban,
  fullLabel: [a.displayIban, ...a.pans].join(' · '),
  theme: a.theme
})
const selectedAccounts = computed(() => preparedAccounts.value.filter(isSelected).map(toRow))

// График — по выбранным картам (ничего не выбрано → все счета, как в Смете). Клик по свече сужает таблицу до её корзины времени.
const pickedIds = computed(() => selectedAccounts.value.map(a => a.id))
const { bondOps, chartTransactions, chartTotalUAH, binanceIdList, payEvents } =
  useCapitalChart(toRef(props, 'accounts'), toRef(props, 'transactions'), pickedIds)

const chartRef = ref(null)
const range = ref(null) // { from, to, title } выбранной свечи
const clearRange = () => { range.value = null; chartRef.value?.clearSelection() }

// Таблица: выбранные карты; если карт нет, но выбрана свеча — все счета в её интервале
const tableAccounts = computed(() =>
  selectedAccounts.value.length ? selectedAccounts.value : range.value ? preparedAccounts.value.map(toRow) : []
)
const allScope = computed(() => !selectedAccounts.value.length && !!range.value)
</script>

<template>
  <main class="space-y-4 font-mono select-none">
    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Подключенные счета</div>
    
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <AccountPanel
        v-for="acc in preparedAccounts"
        :key="acc.key"
        :card="acc"
        selectable
        :selected="isSelected(acc)"
        role="checkbox"
        tabindex="0"
        :aria-checked="isSelected(acc)"
        @click="toggle(acc)"
        @keydown.enter.prevent="toggle(acc)"
        @keydown.space.prevent="toggle(acc)"
      />
    </div>

    <CandleChart
      ref="chartRef"
      selectable
      :transactions="chartTransactions"
      :binance-ids="binanceIdList"
      :total-value-u-a-h="chartTotalUAH"
      :events="payEvents"
      @select="range = $event"
    />

    <AccountHistoryTable
      :users="users"
      :transactions="transactions"
      :memo="bondOps"
      :selected="tableAccounts"
      :all-scope="allScope"
      :range="range"
      @clear="selectedIds = []"
      @clear-range="clearRange"
    />
  </main>
</template>
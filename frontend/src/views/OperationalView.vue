<script setup>
import { computed, ref } from 'vue'
import { prepareAccount } from '../utils/accountView.js'
import AccountPanel from '../components/AccountPanel.vue'
import AccountHistoryTable from '../components/AccountHistoryTable.vue'
import IcuLedgerModal from '../components/IcuLedgerModal.vue'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
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

// ICU: журнал открывается карандашом на карточке (сам счёт появляется, когда IBAN банка ICU привязан к алиасу)
const ledgerId = ref(null)
const ledgerAcc = computed(() => props.accounts.find(a => a.id === ledgerId.value) || null)

const selectedAccounts = computed(() =>
  preparedAccounts.value
    .filter(isSelected)
    .map(a => ({
      id: a.id,
      label: a.pans[0] || a.displayIban,
      fullLabel: [a.displayIban, ...a.pans].join(' · '),
      theme: a.theme
    }))
)
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
        @edit="ledgerId = acc.id"
        @keydown.enter.prevent="toggle(acc)"
        @keydown.space.prevent="toggle(acc)"
      />
    </div>

    <AccountHistoryTable :transactions="transactions" :selected="selectedAccounts" @clear="selectedIds = []" />
    <IcuLedgerModal :account="ledgerAcc" :is-open="!!ledgerAcc" @close="ledgerId = null" />
  </main>
</template>
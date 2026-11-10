<script setup>
import { computed, ref } from 'vue'
import { formatMoney, formatMaskedIBAN, lifeSpan } from '../utils/formatters.js'
import { pansOf, detectBankKey } from '../utils/personLayout.js'
import { bankTheme } from '../utils/bankTheme.js'
import UiDot from '../components/ui/UiDot.vue'
import AccountHistoryTable from '../components/AccountHistoryTable.vue'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})

const preparedAccounts = computed(() => {
  return props.accounts.map(acc => {
    const rawIban = acc.iban || acc.account_identifier || acc.name || ''
    // Все карты счёта: главная первой, остальные (перевыпуск, вторая карта) — следом
    const pans = pansOf(acc).filter(p => p !== rawIban)
    const theme = bankTheme(detectBankKey(rawIban, acc, pans), acc.type)

    return {
      ...acc,
      key: acc.id || acc.iban,
      displayIban: formatMaskedIBAN(rawIban),
      pans: pans.map(formatMaskedIBAN),
      theme
    }
  })
})

// Мультиселект плиток (как в приложении ПриватБанка): клик добавляет/снимает карту, история — по выбранным
const selectedIds = ref([])
const isSelected = (acc) => selectedIds.value.includes(acc.key)
const toggle = (acc) => {
  selectedIds.value = isSelected(acc)
    ? selectedIds.value.filter(id => id !== acc.key)
    : [...selectedIds.value, acc.key]
}

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
      <div
        v-for="acc in preparedAccounts"
        :key="acc.key"
        role="checkbox"
        tabindex="0"
        :aria-checked="isSelected(acc)"
        class="ui-select-card border p-5 rounded-lg flex flex-col justify-between transition-all duration-300 shadow-lg relative overflow-hidden"
        :class="[acc.theme.panel, isSelected(acc) && 'ui-select-card--on', acc.is_closed && 'ui-closed']"
        @click="toggle(acc)"
        @keydown.enter.prevent="toggle(acc)"
        @keydown.space.prevent="toggle(acc)"
      >
        <!-- Шапка: Имя IBAN + Бейдж банка -->
        <div class="flex items-start justify-between gap-3 z-10">
          <div class="space-y-1 min-w-0">
            <div class="flex items-center gap-2 min-w-0">
              <UiDot cached />
              <div class="text-sm font-bold truncate tracking-wide">
                {{ acc.displayIban }}
              </div>
            </div>

            <!-- Привязанная карта (PAN) -->
            <div
              v-for="(pan, i) in acc.pans"
              :key="pan + i"
              class="flex items-center gap-1.5 text-[11px] pl-4"
              :class="i === 0 ? 'opacity-80' : 'opacity-50'"
            >
              <span class="opacity-50 select-none">└─</span>
              <span class="tracking-wider">{{ pan }}</span>
            </div>
          </div>

          <div class="flex flex-col items-end gap-1 shrink-0">
            <span
              class="text-[10px] font-bold px-2 py-0.5 rounded border uppercase font-mono"
              :class="acc.theme.tagStyle"
            >
              {{ acc.theme.tag }}
            </span>
            <span v-if="acc.is_closed" class="ui-closed-tag">Закрыта</span>
          </div>
        </div>

        <!-- Нижний блок: Баланс -->
        <div class="mt-6 space-y-1 z-10">
          <span class="text-[10px] opacity-60 uppercase block font-bold tracking-wider">Баланс счета:</span>
          <span class="text-xl font-bold tracking-wide">{{ formatMoney(acc.balance) }} ₴</span>
          <span v-if="lifeSpan(acc)" class="text-[10px] opacity-60 block tracking-wider">{{ lifeSpan(acc) }}</span>
        </div>

        <!-- Отметка выбора -->
        <span class="ui-select-mark" :class="isSelected(acc) && 'ui-select-mark--on'">
          <svg v-if="isSelected(acc)" viewBox="0 0 24 24" class="w-3 h-3" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12l5 5L20 7" /></svg>
        </span>
      </div>
    </div>

    <AccountHistoryTable :transactions="transactions" :selected="selectedAccounts" @clear="selectedIds = []" />
  </main>
</template>
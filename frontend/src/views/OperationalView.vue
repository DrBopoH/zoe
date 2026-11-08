<script setup>
import { formatMoney, cleanAccountName } from '../utils/formatters.js'

defineProps({
  accounts: { type: Array, default: () => [] }
})
</script>

<template>
  <main class="space-y-4">
    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Подключенные счета</div>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="acc in accounts"
        :key="acc.id"
        class="bg-slate-900/60 border border-slate-800 p-5 rounded flex flex-col justify-between"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="text-sm font-bold text-slate-200 truncate">
            {{ cleanAccountName(acc.name) }}
          </div>
          <span class="text-[10px] font-bold px-2 py-0.5 rounded border bg-slate-800/80 text-emerald-400 border-slate-700 uppercase shrink-0">
            {{ (acc.type || '').toUpperCase() }}
          </span>
        </div>
        
        <div class="mt-6 space-y-1">
          <span class="text-[10px] text-slate-500 uppercase block">Баланс счета:</span>
          <span class="text-xl font-bold text-white">{{ formatMoney(acc.balance) }} ₴</span>
        </div>
      </div>
    </div>
  </main>
</template>
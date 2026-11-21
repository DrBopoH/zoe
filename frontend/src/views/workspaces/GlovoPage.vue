<script setup>
import { ref } from 'vue'
import WorkspaceLedger from '../../components/WorkspaceLedger.vue'
import UiEmpty from '../../components/ui/UiEmpty.vue'

// Страница дела «Glovo»: две части. «Учёт» — счета дела, бейджи на операциях (Расход, Убыток, Чай, Доход, Выплата) и понедельный номинальный доход.
// «Смета» — графики со сведением всех данных дела; пока пустая. Шапку с названием и обложкой рисует WorkspaceView.
defineProps({
  workspace: { type: Object, required: true },
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] }
})

const PARTS = [
  { key: 'ledger', label: 'Учёт' },
  { key: 'estimate', label: 'Смета' }
]
const part = ref('ledger')
</script>

<template>
  <div class="space-y-4">
    <div class="flex gap-1 border-b border-slate-800" role="tablist">
      <button
        v-for="p in PARTS"
        :key="p.key"
        type="button"
        role="tab"
        :aria-selected="part === p.key"
        class="px-4 py-2 text-xs font-bold uppercase tracking-wider cursor-pointer border-b-2 -mb-px transition-colors"
        :class="part === p.key ? 'border-emerald-400 text-emerald-400' : 'border-transparent text-slate-500 hover:text-slate-300'"
        @click="part = p.key"
      >{{ p.label }}</button>
    </div>

    <WorkspaceLedger v-if="part === 'ledger'" :workspace-id="workspace.id" :accounts="accounts" :transactions="transactions" :users="users" />
    <UiEmpty v-else>Смета пока пустая: здесь будут графики со сведением всех данных дела</UiEmpty>
  </div>
</template>

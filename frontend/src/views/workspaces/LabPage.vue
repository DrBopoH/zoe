<script setup>
import { ref } from 'vue'
import WorkspaceLedger from '../../components/WorkspaceLedger.vue'
import LabStock from '../../components/LabStock.vue'
import UiEmpty from '../../components/ui/UiEmpty.vue'
import { LAB_BADGES } from '../../utils/labLedger.js'

// Страница дела «ХимЛаб» (производство напитков): четыре части.
// «Учёт» — счета, бейджи Расход / Убыток / Продажа и наличные записи (общий механизм со страницей Glovo).
// «Склад» — вещества и их карточки с запасом от номинала до расчётного; «Партии» (производство и себестоимость) и «Смета» — следующие шаги.
// Шапку с названием и обложкой рисует WorkspaceView.
defineProps({
  workspace: { type: Object, required: true },
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] }
})

const PARTS = [
  { key: 'ledger', label: 'Учёт' },
  { key: 'stock', label: 'Склад' },
  { key: 'batches', label: 'Партии', hint: 'Партии появятся после склада: номер партии, рецепт на штуку и себестоимость.' },
  { key: 'estimate', label: 'Смета', hint: 'Смета лаба появится, когда будут склад и партии: стоимость склада, себестоимость штуки, выручка, потери.' }
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

    <WorkspaceLedger v-if="part === 'ledger'" :workspace-id="workspace.id" :accounts="accounts" :transactions="transactions" :users="users" :badges="LAB_BADGES" :nominal="false" product-cards />
    <LabStock v-else-if="part === 'stock'" :workspace-id="workspace.id" />
    <UiEmpty v-else>{{ PARTS.find((p) => p.key === part).hint }}</UiEmpty>
  </div>
</template>

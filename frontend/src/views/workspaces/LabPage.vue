<script setup>
import { computed, ref, watch, onMounted } from 'vue'
import WorkspaceLedger from '../../components/WorkspaceLedger.vue'
import LabStock from '../../components/LabStock.vue'
import LabBatches from '../../components/LabBatches.vue'
import UiLoadError from '../../components/ui/UiLoadError.vue'
import { useWorkspaceDocs } from '../../composables/useWorkspaceDocs.js'
import { pushRequestError } from '../../composables/useToasts.js'
import UiEmpty from '../../components/ui/UiEmpty.vue'
import { LAB_BADGES, CARD_KIND, BATCH_KIND, batchUsage } from '../../utils/labLedger.js'

// Страница дела «ХимЛаб» (производство напитков): четыре части.
// «Учёт» — счета, бейджи Расход / Убыток / Продажа и наличные записи (общий механизм со страницей Glovo).
// «Склад» — вещества и их карточки с запасом от номинала до расчётного (минус партии); «Партии» — производство и себестоимость; «Смета» — следующий шаг.
// Шапку с названием и обложкой рисует WorkspaceView.
const props = defineProps({
  workspace: { type: Object, required: true },
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] }
})

const PARTS = [
  { key: 'ledger', label: 'Учёт' },
  { key: 'stock', label: 'Склад' },
  { key: 'batches', label: 'Партии' },
  { key: 'estimate', label: 'Смета', hint: 'Смета лаба появится, когда будут склад и партии: стоимость склада, себестоимость штуки, выручка, потери.' }
]
const part = ref('ledger')

// Карточки и партии нужны Складу и Партиям; перечитываются при входе в эти части
const cardDocs = useWorkspaceDocs(() => props.workspace.id, CARD_KIND)
const batchDocs = useWorkspaceDocs(() => props.workspace.id, BATCH_KIND)
const reload = () => Promise.all([cardDocs.load(), batchDocs.load()])
watch(part, (p) => { if (p === 'stock' || p === 'batches') reload() })
onMounted(() => { if (part.value !== 'ledger') reload() })
const error = computed(() => cardDocs.error.value || batchDocs.error.value)
const used = computed(() => batchUsage(batchDocs.docs.value))

const onSaveBatch = async (b) => {
  try { await (b.id ? batchDocs.update(b) : batchDocs.add(b)); return true } catch (e) { pushRequestError('Не удалось сохранить партию', e); return false }
}
const onRemoveBatch = async (id) => {
  try { await batchDocs.remove(id) } catch (e) { pushRequestError('Не удалось удалить партию', e) }
}
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
    <template v-else-if="part === 'stock' || part === 'batches'">
      <UiLoadError v-if="error" :message="error" @retry="reload" />
      <LabStock v-if="part === 'stock'" :cards="cardDocs.docs.value" :used="used" />
      <LabBatches v-else :batches="batchDocs.docs.value" :cards="cardDocs.docs.value" :save="onSaveBatch" :remove="onRemoveBatch" />
    </template>
    <UiEmpty v-else>{{ PARTS.find((p) => p.key === part).hint }}</UiEmpty>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted } from 'vue'
import WorkspaceLedger from '../../components/WorkspaceLedger.vue'
import LabStock from '../../components/LabStock.vue'
import LabBatches from '../../components/LabBatches.vue'
import LabEstimate from '../../components/LabEstimate.vue'
import UiLoadError from '../../components/ui/UiLoadError.vue'
import { toLocalDateStr } from '../../utils/formatters.js'
import { useWorkspaces } from '../../composables/useWorkspaces.js'
import { apiJson } from '../../utils/api.js'
import { useWorkspaceDocs } from '../../composables/useWorkspaceDocs.js'
import { pushRequestError } from '../../composables/useToasts.js'
import { LAB_BADGES, CARD_KIND, BATCH_KIND, MOVE_KIND, WRITEOFF_KIND, batchUsage, batchCost, mergeUsage, writeoffUsage, writeoffAmount } from '../../utils/labLedger.js'

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
  { key: 'estimate', label: 'Смета' }
]
const part = ref('ledger')

// Карточки и партии нужны Складу и Партиям; перечитываются при входе в эти части
const cardDocs = useWorkspaceDocs(() => props.workspace.id, CARD_KIND)
const batchDocs = useWorkspaceDocs(() => props.workspace.id, BATCH_KIND)
const moveDocs = useWorkspaceDocs(() => props.workspace.id, MOVE_KIND)
const writeoffDocs = useWorkspaceDocs(() => props.workspace.id, WRITEOFF_KIND)
const allWorkspaces = useWorkspaces()
const others = computed(() => allWorkspaces.list.value.filter((w) => w.id !== props.workspace.id))
const reload = () => Promise.all([cardDocs.load(), batchDocs.load(), moveDocs.load(), writeoffDocs.load(), allWorkspaces.load()])
watch(part, (p) => { if (p !== 'ledger') reload() })
onMounted(() => { if (part.value !== 'ledger') reload() })
const error = computed(() => cardDocs.error.value || batchDocs.error.value || moveDocs.error.value || writeoffDocs.error.value)
const written = computed(() => writeoffUsage(writeoffDocs.docs.value))
const used = computed(() => mergeUsage(batchUsage(batchDocs.docs.value), written.value))

const onSaveBatch = async (b) => {
  try { await (b.id ? batchDocs.update(b) : batchDocs.add(b)); return true } catch (e) { pushRequestError('Не удалось сохранить партию', e); return false }
}
// Продажа — просто запись с ценой. Сжигание ещё и пишет расход в выбранное дело (запись с источником — id этого движения):
// она рисуется там в истории между операциями и уходит вместе с движением. Не вышло записать расход — движение откатывается.
const onSaveMove = async (m) => {
  try {
    const batch = batchDocs.docs.value.find((b) => b.id === m.batch_id)
    const doc = await moveDocs.add(m)
    if (m.type === 'burn') {
      const unit = batch && batch.pieces ? batchCost(batch, cardDocs.docs.value).cost / batch.pieces : 0
      try {
        await apiJson('POST', '/workspaces/entries', { workspace_id: m.target_ws, day: m.day, badge: 'expense', amount: Math.round(unit * m.pieces * 100) / 100, note: `ХимЛаб · ${batch ? batch.code : ''} · ${m.pieces} шт`, tx_id: '', src: doc.id })
      } catch (e) {
        await moveDocs.remove(doc.id).catch(() => {})
        throw e
      }
    }
    return true
  } catch (e) { pushRequestError('Не удалось сохранить движение', e); return false }
}
// Списать остаток карточки: документ-списание и убыток в «Учёте» ХимЛаба (запись с источником — id списания, только для чтения);
// не записался убыток — списание откатывается. Отмена списания возвращает остаток и убирает убыток.
const onWriteoff = async (card, grams) => {
  try {
    const doc = await writeoffDocs.add({ day: toLocalDateStr(new Date()), card_id: card.id, grams })
    try {
      await apiJson('POST', '/workspaces/entries', { workspace_id: props.workspace.id, day: doc.day, badge: 'loss', amount: writeoffAmount(doc, card), note: `Списание остатка · ${card.substance}${card.maker ? ' · ' + card.maker : ''}`.slice(0, 200), tx_id: '', src: doc.id })
    } catch (e) {
      await writeoffDocs.remove(doc.id).catch(() => {})
      throw e
    }
    return true
  } catch (e) { pushRequestError('Не удалось списать остаток', e); return false }
}
const onUnwriteoff = async (id) => {
  try { await writeoffDocs.remove(id) } catch (e) { pushRequestError('Не удалось отменить списание', e) }
}
const onRemoveMove = async (id) => {
  try { await moveDocs.remove(id) } catch (e) { pushRequestError('Не удалось удалить движение', e) }
}
const onRemoveBatch = async (id) => {
  try {
    // движения партии уходят вместе с ней (расход в других делах — тоже)
    for (const m of moveDocs.docs.value.filter((x) => x.batch_id === id)) await moveDocs.remove(m.id)
    await batchDocs.remove(id)
  } catch (e) { pushRequestError('Не удалось удалить партию', e) }
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
    <template v-else-if="part === 'stock' || part === 'batches' || part === 'estimate'">
      <UiLoadError v-if="error" :message="error" @retry="reload" />
      <LabStock v-if="part === 'stock'" :cards="cardDocs.docs.value" :used="used" :writeoffs="writeoffDocs.docs.value" :write="onWriteoff" :unwrite="onUnwriteoff" />
      <LabBatches v-else-if="part === 'batches'" :batches="batchDocs.docs.value" :cards="cardDocs.docs.value" :save="onSaveBatch" :remove="onRemoveBatch" :moves="moveDocs.docs.value" :workspaces="others" :save-move="onSaveMove" :remove-move="onRemoveMove" :written="written" />
      <LabEstimate v-else :workspace-id="workspace.id" :transactions="transactions" :cards="cardDocs.docs.value" :batches="batchDocs.docs.value" :moves="moveDocs.docs.value" :writeoffs="writeoffDocs.docs.value" />
    </template>
  </div>
</template>

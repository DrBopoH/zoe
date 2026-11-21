<script setup>
import { computed, ref } from 'vue'
import UiButton from './ui/UiButton.vue'
import UiEmpty from './ui/UiEmpty.vue'
import UiIconButton from './ui/UiIconButton.vue'
import LabBatchForm from './LabBatchForm.vue'
import LabMoveForm from './LabMoveForm.vue'
import { batchCost, batchPieces, moveAmounts } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'
import { numText, dmy } from '../utils/numfmt.js'

// Партии производства («транзакции производства»): номер, дата, штуки, себестоимость партии и штуки, состав по карточкам.
const props = defineProps({
  batches: { type: Array, default: () => [] },
  cards: { type: Array, default: () => [] },
  save: { type: Function, required: true }, // (партия) → true/false
  remove: { type: Function, required: true },
  // движения партий (продажи и сжигания), другие дела для сжигания, их сохранение и удаление
  moves: { type: Array, default: () => [] },
  workspaces: { type: Array, default: () => [] },
  saveMove: { type: Function, default: null }, // (движение) → true/false
  removeMove: { type: Function, default: null },
  written: { type: Object, default: () => new Map() }
})

const form = ref(null) // 'new' | id партии
const moveForm = ref(null) // { batch id, type }
const armedMove = ref('')
const busy = ref(false)
const open = ref('')
const armed = ref('')
let timer
const list = computed(() => [...props.batches].sort((a, b) => (a.day < b.day ? 1 : a.day > b.day ? -1 : a.code < b.code ? 1 : -1)).map((b) => ({ b, c: batchCost(b, props.cards), p: batchPieces(b, props.moves), mv: props.moves.filter((m) => m.batch_id === b.id).sort((x, y) => (x.day < y.day ? 1 : -1)) })))

const onSave = async (b) => {
  busy.value = true
  try { if (await props.save(b)) form.value = null } finally { busy.value = false }
}
const trash = (b) => {
  if (armed.value === b.id) { clearTimeout(timer); armed.value = ''; props.remove(b.id); return }
  armed.value = b.id
  clearTimeout(timer)
  timer = setTimeout(() => { armed.value = '' }, 4000)
}
const onMove = async (m) => {
  busy.value = true
  try { if (await props.saveMove(m)) moveForm.value = null } finally { busy.value = false }
}
const trashMove = (m) => {
  if (armedMove.value === m.id) { clearTimeout(timer); armedMove.value = ''; props.removeMove(m.id); return }
  armedMove.value = m.id
  clearTimeout(timer)
  timer = setTimeout(() => { armedMove.value = '' }, 4000)
}
const amounts = (m, b) => moveAmounts(m, b, props.cards)
const cardName = (l) => (l.card ? `${l.card.substance} · ${l.card.maker || 'без производителя'} · ${numText(l.card.net_g, 3)} г` : 'карточка удалена')
</script>

<template>
  <div class="space-y-4 font-mono select-none" data-testid="lab-batches">
    <div class="flex items-center justify-between">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Партии</div>
      <UiButton v-if="form !== 'new'" variant="primary" size="sm" data-testid="batch-new" @click="form = 'new'">+ Партия</UiButton>
    </div>

    <LabBatchForm v-if="form === 'new'" :cards="cards" :batches="batches" :written="written" :busy="busy" @save="onSave" @cancel="form = null" />

    <UiEmpty v-if="!list.length && form !== 'new'">Партий пока нет. Партия списывает граммы с конкретных карточек склада и считает себестоимость.</UiEmpty>

    <div v-for="{ b, c, p, mv } in list" :key="b.id" class="rounded border border-slate-800 bg-slate-900/60" data-testid="batch">
      <LabBatchForm v-if="form === b.id" :batch="b" :cards="cards" :batches="batches" :written="written" :busy="busy" @save="onSave" @cancel="form = null" />
      <template v-else>
        <div class="group flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2 cursor-pointer hover:bg-slate-800/40" @click="open = open === b.id ? '' : b.id">
          <span class="text-sm font-bold text-slate-200 tabular-nums">{{ b.code }}</span>
          <span class="text-[11px] text-slate-500">{{ dmy(b.day) }}</span>
          <span v-if="b.note" class="text-[11px] text-slate-400 truncate">{{ b.note }}</span>
          <span class="ml-auto text-[11px] text-slate-400 tabular-nums">
            <template v-if="b.pieces"><span data-testid="batch-left">{{ p.left }}</span> из {{ b.pieces }} шт · </template>
            <span class="text-slate-200 font-bold" data-testid="batch-cost">{{ formatMoney(c.cost) }}</span>
            <template v-if="c.perPiece !== null"> · <span class="text-emerald-400 font-bold" data-testid="batch-piece">{{ formatMoney(c.perPiece) }}</span> за шт</template>
          </span>
          <span class="inline-flex items-center gap-0.5 transition-opacity" :class="armed === b.id ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'">
            <span v-if="armed === b.id" class="text-[10px] font-bold uppercase tracking-wider text-rose-300 mr-1">Удалить?</span>
            <UiIconButton v-else icon="edit" tone="accent" size="sm" title="Править партию" data-testid="batch-edit" @click.stop="form = b.id" />
            <UiIconButton icon="trash" tone="danger" size="sm" :title="armed === b.id ? 'Нажми ещё раз — удалить' : 'Удалить партию'" data-testid="batch-remove" @click.stop="trash(b)" />
          </span>
        </div>
        <ul v-if="open === b.id" class="border-t border-slate-800 px-3 py-2 space-y-1 text-[11px]" data-testid="batch-lines">
          <li v-for="(l, i) in c.lines" :key="i" class="flex flex-wrap justify-between gap-x-4">
            <span :class="l.missing ? 'text-rose-300' : 'text-slate-300'">{{ cardName(l) }}</span>
            <span class="text-slate-400 tabular-nums">
              {{ numText(l.grams, 3) }} г<template v-if="b.pieces"> ({{ numText(l.grams / b.pieces, 3) }} г/шт)</template> · {{ formatMoney(l.cost) }}
            </span>
          </li>
        </ul>
        <div v-if="open === b.id" class="border-t border-slate-800 px-3 py-2 space-y-2" data-testid="batch-moves">
          <ul v-if="mv.length" class="space-y-1 text-[11px]">
            <li v-for="m in mv" :key="m.id" class="group flex flex-wrap items-center gap-x-3" data-testid="move-row">
              <span class="text-[9px] px-1.5 py-0.5 rounded border font-bold" :class="m.type === 'sale' ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' : 'bg-orange-500/15 text-orange-300 border-orange-500/30'">{{ m.type === 'sale' ? 'Продажа' : 'Сжигание' }}</span>
              <span class="text-slate-500">{{ dmy(m.day) }}</span>
              <span class="text-slate-300 tabular-nums">{{ m.pieces }} шт<template v-if="m.type === 'sale'"> × {{ formatMoney(m.price) }}</template><template v-else> → «{{ m.target_name }}»</template></span>
              <span v-if="m.note" class="text-slate-500 truncate">{{ m.note }}</span>
              <span class="ml-auto tabular-nums text-slate-400">
                <template v-if="m.type === 'sale'">{{ formatMoney(amounts(m, b).revenue) }} · наценка <span :class="amounts(m, b).margin >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ formatMoney(amounts(m, b).margin) }}</span></template>
                <template v-else>{{ formatMoney(amounts(m, b).cost) }} по себестоимости</template>
              </span>
              <span v-if="removeMove" class="inline-flex items-center gap-0.5 transition-opacity" :class="armedMove === m.id ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'">
                <span v-if="armedMove === m.id" class="text-[10px] font-bold uppercase tracking-wider text-rose-300 mr-1">Удалить?</span>
                <UiIconButton icon="trash" tone="danger" size="sm" title="Удалить движение (расход в другом деле уйдёт вместе с ним)" data-testid="move-remove" @click.stop="trashMove(m)" />
              </span>
            </li>
          </ul>
          <template v-if="saveMove">
            <LabMoveForm v-if="moveForm && moveForm.id === b.id" :type="moveForm.type" :batch="b" :left="p.left" :unit-cost="b.pieces ? c.cost / b.pieces : 0" :workspaces="workspaces" :busy="busy" @save="onMove" @cancel="moveForm = null" />
            <div v-else-if="b.pieces" class="flex gap-4">
              <button type="button" class="text-[11px] font-bold text-slate-500 hover:text-emerald-300 transition-colors cursor-pointer" data-testid="move-sale" @click.stop="moveForm = { id: b.id, type: 'sale' }">+ Продать</button>
              <button type="button" class="text-[11px] font-bold text-slate-500 hover:text-orange-300 transition-colors cursor-pointer" data-testid="move-burn" @click.stop="moveForm = { id: b.id, type: 'burn' }">+ Сжечь</button>
            </div>
            <p v-else class="text-[11px] text-slate-500">Укажи число штук в партии (карандаш), чтобы продавать и сжигать.</p>
          </template>
        </div>
      </template>
    </div>
  </div>
</template>

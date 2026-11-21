<script setup>
import { computed, ref } from 'vue'
import UiButton from './ui/UiButton.vue'
import UiEmpty from './ui/UiEmpty.vue'
import UiIconButton from './ui/UiIconButton.vue'
import LabBatchForm from './LabBatchForm.vue'
import { batchCost } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'
import { numText, dmy } from '../utils/numfmt.js'

// Партии производства («транзакции производства»): номер, дата, штуки, себестоимость партии и штуки, состав по карточкам.
const props = defineProps({
  batches: { type: Array, default: () => [] },
  cards: { type: Array, default: () => [] },
  save: { type: Function, required: true }, // (партия) → true/false
  remove: { type: Function, required: true }
})

const form = ref(null) // 'new' | id партии
const busy = ref(false)
const open = ref('')
const armed = ref('')
let timer
const list = computed(() => [...props.batches].sort((a, b) => (a.day < b.day ? 1 : a.day > b.day ? -1 : a.code < b.code ? 1 : -1)).map((b) => ({ b, c: batchCost(b, props.cards) })))

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
const cardName = (l) => (l.card ? `${l.card.substance} · ${l.card.maker || 'без производителя'} · ${numText(l.card.net_g, 1)} г` : 'карточка удалена')
</script>

<template>
  <div class="space-y-4 font-mono select-none" data-testid="lab-batches">
    <div class="flex items-center justify-between">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Партии</div>
      <UiButton v-if="form !== 'new'" variant="primary" size="sm" data-testid="batch-new" @click="form = 'new'">+ Партия</UiButton>
    </div>

    <LabBatchForm v-if="form === 'new'" :cards="cards" :batches="batches" :busy="busy" @save="onSave" @cancel="form = null" />

    <UiEmpty v-if="!list.length && form !== 'new'">Партий пока нет. Партия списывает граммы с конкретных карточек склада и считает себестоимость.</UiEmpty>

    <div v-for="{ b, c } in list" :key="b.id" class="rounded border border-slate-800 bg-slate-900/60" data-testid="batch">
      <LabBatchForm v-if="form === b.id" :batch="b" :cards="cards" :batches="batches" :busy="busy" @save="onSave" @cancel="form = null" />
      <template v-else>
        <div class="group flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2 cursor-pointer hover:bg-slate-800/40" @click="open = open === b.id ? '' : b.id">
          <span class="text-sm font-bold text-slate-200 tabular-nums">{{ b.code }}</span>
          <span class="text-[11px] text-slate-500">{{ dmy(b.day) }}</span>
          <span v-if="b.note" class="text-[11px] text-slate-400 truncate">{{ b.note }}</span>
          <span class="ml-auto text-[11px] text-slate-400 tabular-nums">
            <template v-if="b.pieces">{{ b.pieces }} шт · </template>
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
              {{ numText(l.grams, 2) }} г<template v-if="b.pieces"> ({{ numText(l.grams / b.pieces, 2) }} г/шт)</template> · {{ formatMoney(l.cost) }}
            </span>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>

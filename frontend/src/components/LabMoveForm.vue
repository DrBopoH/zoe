<script setup>
import { ref, computed } from 'vue'
import UiButton from './ui/UiButton.vue'
import { cleanMove } from '../utils/labLedger.js'
import { formatMoney, toLocalDateStr } from '../utils/formatters.js'

// Форма движения партии: продажа (штуки и цена за штуку) или сжигание (штуки и дело, куда пойдёт расход по себестоимости).
const props = defineProps({
  type: { type: String, required: true }, // 'sale' | 'burn'
  batch: { type: Object, required: true },
  left: { type: Number, required: true }, // сколько штук ещё в наличии
  unitCost: { type: Number, default: 0 }, // себестоимость штуки
  workspaces: { type: Array, default: () => [] }, // другие дела: { id, name, color }
  busy: { type: Boolean, default: false }
})
const emit = defineEmits(['save', 'cancel'])

const f = ref({ day: toLocalDateStr(new Date()), pieces: '1', price: '', target_ws: props.workspaces[0]?.id || '', note: '' })
const target = computed(() => props.workspaces.find((w) => w.id === f.value.target_ws) || null)
const clean = computed(() => cleanMove({ ...f.value, type: props.type, batch_id: props.batch.id, target_name: target.value?.name || '' }))
const over = computed(() => !!clean.value && clean.value.pieces > props.left)
const amount = computed(() => (clean.value ? Math.round(clean.value.pieces * (props.type === 'sale' ? clean.value.price : props.unitCost) * 100) / 100 : null))
const can = computed(() => !!clean.value && !over.value && !props.busy && (props.type === 'sale' || amount.value > 0))
const submit = () => { if (can.value) emit('save', clean.value) }
</script>

<template>
  <div class="space-y-2 rounded-md border border-slate-700/60 bg-slate-950/60 p-3 select-none" data-testid="move-form" @click.stop>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
      <label class="space-y-1">
        <span class="ui-label">Дата</span>
        <input v-model="f.day" type="date" class="ui-input" data-field="day" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Штук <span class="text-slate-500 normal-case">(в наличии {{ left }})</span></span>
        <input v-model="f.pieces" type="number" min="1" step="1" class="ui-input tabular-nums" data-field="pieces" @keydown.enter.prevent="submit" />
      </label>
      <label v-if="type === 'sale'" class="space-y-1">
        <span class="ui-label">Цена за штуку, ₴</span>
        <input v-model="f.price" type="number" min="0" step="0.01" class="ui-input tabular-nums" data-field="price" @keydown.enter.prevent="submit" />
      </label>
      <label v-else class="space-y-1">
        <span class="ui-label">В деле</span>
        <select v-model="f.target_ws" class="ui-input" data-field="target">
          <option v-for="w in workspaces" :key="w.id" :value="w.id">{{ w.name }}</option>
        </select>
      </label>
      <label class="space-y-1">
        <span class="ui-label">Пометка</span>
        <input v-model="f.note" type="text" class="ui-input" maxlength="100" data-field="note" @keydown.enter.prevent="submit" />
      </label>
    </div>
    <p v-if="over" class="text-[11px] text-rose-300" data-testid="move-over">В наличии только {{ left }} шт.</p>
    <p v-if="type === 'burn' && !workspaces.length" class="text-[11px] text-amber-300">Других дел пока нет: создай дело, в котором продукция пойдёт в расход.</p>
    <div class="flex flex-wrap items-center justify-end gap-3">
      <span v-if="amount !== null" class="text-[11px] text-slate-400 mr-auto tabular-nums" data-testid="move-amount">
        <template v-if="type === 'sale'">Выручка <span class="text-slate-200 font-bold">{{ formatMoney(amount) }}</span> · наценка <span class="font-bold" :class="amount - clean.pieces * unitCost >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ formatMoney(Math.round((amount - clean.pieces * unitCost) * 100) / 100) }}</span></template>
        <template v-else>По себестоимости <span class="text-slate-200 font-bold">{{ formatMoney(amount) }}</span>: расход в деле «{{ target?.name }}», компенсация лабу</template>
      </span>
      <UiButton variant="cancel" size="sm" @click="emit('cancel')">Отмена</UiButton>
      <UiButton variant="primary" size="sm" :disabled="!can" @click="submit">{{ busy ? 'Сохраняю…' : type === 'sale' ? 'Продать' : 'Сжечь' }}</UiButton>
    </div>
  </div>
</template>

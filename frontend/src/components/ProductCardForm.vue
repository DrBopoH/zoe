<script setup>
import { ref, computed, watch } from 'vue'
import UiButton from './ui/UiButton.vue'
import { cleanCard, cardTotal } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'

// Форма карточки товара под операцией «Расход» в ХимЛабе: вещество, производитель, нетто фасовки, цена за штуку и число штук.
// Подсказки веществ и производителей растут из уже введённых карточек.
const props = defineProps({
  card: { type: Object, default: null }, // для правки; без неё форма добавляет новую
  txId: { type: String, required: true },
  day: { type: String, required: true },
  busy: { type: Boolean, default: false },
  substances: { type: Array, default: () => [] },
  makers: { type: Array, default: () => [] }
})
const emit = defineEmits(['save', 'cancel'])

const f = ref({ substance: '', maker: '', net_g: '', price: '', qty: '1', note: '' })
watch(() => props.card, (c) => {
  f.value = c
    ? { substance: c.substance, maker: c.maker || '', net_g: String(c.net_g), price: String(c.price), qty: String(c.qty), note: c.note || '' }
    : { substance: '', maker: '', net_g: '', price: '', qty: '1', note: '' }
}, { immediate: true })

const clean = computed(() => cleanCard(f.value))
const can = computed(() => !!clean.value && !props.busy)
const total = computed(() => (clean.value ? cardTotal(clean.value) : null))

const submit = () => {
  if (!can.value) return
  emit('save', { ...(props.card ? { id: props.card.id } : {}), day: props.card?.day || props.day, tx_id: props.txId, ...clean.value })
}
</script>

<template>
  <div class="space-y-2 rounded-md border border-slate-700/60 bg-slate-900/70 p-3 select-none" data-testid="card-form" @click.stop>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <label class="space-y-1">
        <span class="ui-label">Вещество</span>
        <input v-model="f.substance" type="text" list="lab-substances" class="ui-input" maxlength="80" data-field="substance" placeholder="мальтодекстрин" @keydown.enter.prevent="submit" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Производитель <span class="text-slate-500 normal-case">(необязательно)</span></span>
        <input v-model="f.maker" type="text" list="lab-makers" class="ui-input" maxlength="80" data-field="maker" @keydown.enter.prevent="submit" />
      </label>
    </div>
    <div class="grid grid-cols-3 gap-2">
      <label class="space-y-1">
        <span class="ui-label">Нетто, г</span>
        <input v-model="f.net_g" type="number" min="0" step="any" class="ui-input tabular-nums" data-field="net_g" placeholder="1000" @keydown.enter.prevent="submit" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Цена за штуку, ₴</span>
        <input v-model="f.price" type="number" min="0" step="0.01" class="ui-input tabular-nums" data-field="price" placeholder="96" @keydown.enter.prevent="submit" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Штук</span>
        <input v-model="f.qty" type="number" min="1" step="1" class="ui-input tabular-nums" data-field="qty" @keydown.enter.prevent="submit" />
      </label>
    </div>
    <label class="block space-y-1">
      <span class="ui-label">Пометка</span>
      <input v-model="f.note" type="text" class="ui-input" maxlength="200" data-field="note" @keydown.enter.prevent="submit" />
    </label>
    <datalist id="lab-substances"><option v-for="s in substances" :key="s" :value="s" /></datalist>
    <datalist id="lab-makers"><option v-for="s in makers" :key="s" :value="s" /></datalist>
    <div class="flex items-center justify-end gap-3">
      <span v-if="total !== null" class="text-[11px] text-slate-400 mr-auto" data-testid="card-total">Всего {{ formatMoney(total) }}</span>
      <UiButton variant="cancel" size="sm" @click="emit('cancel')">Отмена</UiButton>
      <UiButton variant="primary" size="sm" :disabled="!can" @click="submit">{{ busy ? 'Сохраняю…' : card ? 'Сохранить' : 'Добавить' }}</UiButton>
    </div>
  </div>
</template>

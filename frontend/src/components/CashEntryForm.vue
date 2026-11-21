<script setup>
import { ref, computed, watch } from 'vue'
import UiButton from './ui/UiButton.vue'
import { BADGES } from '../utils/glovoLedger.js'

// Форма наличной записи прямо в истории (как форма справочника облигаций): бейдж, сумма и пометка. Дата и операция заданы родителем.
// Сумма всегда положительная, приход это или уход решает бейдж.
const props = defineProps({
  entry: { type: Object, default: null }, // запись для правки; без неё форма добавляет новую
  txId: { type: String, required: true },
  day: { type: String, required: true }, // дата операции, к которой запись прикреплена
  busy: { type: Boolean, default: false }
})
const emit = defineEmits(['save', 'cancel'])

const badge = ref('expense')
const amount = ref('')
const note = ref('')
watch(() => props.entry, (e) => {
  badge.value = e?.badge || 'expense'
  amount.value = e ? String(e.amount) : ''
  note.value = e?.note || ''
}, { immediate: true })

const num = computed(() => Number(String(amount.value).replace(',', '.')))
const can = computed(() => Number.isFinite(num.value) && num.value > 0 && !props.busy)
const side = computed(() => BADGES.find((b) => b.key === badge.value)?.side)

const submit = () => {
  if (!can.value) return
  emit('save', { ...(props.entry ? { id: props.entry.id } : {}), day: props.entry?.day || props.day, badge: badge.value, amount: Math.round(num.value * 100) / 100, note: note.value.trim(), tx_id: props.txId })
}
</script>

<template>
  <div class="space-y-2 rounded border border-slate-800 bg-slate-950/40 p-2 select-none" data-testid="cash-form" @click.stop>
    <div class="flex flex-wrap gap-1.5" data-testid="cash-badges">
      <button
        v-for="b in BADGES"
        :key="b.key"
        type="button"
        class="px-2 py-0.5 rounded border text-[10px] font-bold cursor-pointer transition-opacity"
        :class="[b.cls, badge === b.key ? 'ring-1 ring-white/70' : 'opacity-60 hover:opacity-100']"
        :data-badge="b.key"
        @click="badge = b.key"
      >{{ b.label }}</button>
    </div>
    <div class="grid grid-cols-[8rem_1fr] gap-2">
      <label class="space-y-1">
        <span class="ui-label">Сумма, ₴ <span class="text-slate-500 normal-case">{{ side === 'out' ? '(уход)' : '(приход)' }}</span></span>
        <input v-model="amount" type="number" min="0" step="0.01" class="ui-input tabular-nums" data-field="amount" placeholder="600" @keydown.enter.prevent="submit" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Пометка</span>
        <input v-model="note" type="text" class="ui-input" maxlength="200" data-field="note" placeholder="Например: парафиновое масло, наличкой" @keydown.enter.prevent="submit" />
      </label>
    </div>
    <div class="flex justify-end gap-2">
      <UiButton variant="cancel" size="sm" @click="emit('cancel')">Отмена</UiButton>
      <UiButton variant="primary" size="sm" :disabled="!can" @click="submit">{{ busy ? 'Сохраняю…' : entry ? 'Сохранить' : 'Добавить' }}</UiButton>
    </div>
  </div>
</template>

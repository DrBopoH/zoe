<script setup>
import { ref } from 'vue'
import UiIconButton from './ui/UiIconButton.vue'
import { badgeOf, entrySigned } from '../utils/glovoLedger.js'
import { formatMoney } from '../utils/formatters.js'

// Наличные записи без операции в истории: привязка потерялась (операция вышла из кеша или счёт убран из дела) либо запись старая.
// Они всё равно входят в итоги и смету, а здесь их можно увидеть и удалить (второе нажатие на урну в течение четырёх секунд).
const props = defineProps({
  entries: { type: Array, default: () => [] },
  badges: { type: Array, required: true }
})
const emit = defineEmits(['remove'])

const armedId = ref('')
let timer
const onTrash = (e) => {
  if (armedId.value === e.id) { clearTimeout(timer); armedId.value = ''; emit('remove', e.id); return }
  armedId.value = e.id
  clearTimeout(timer)
  timer = setTimeout(() => { armedId.value = '' }, 4000)
}
const dmy = (d) => `${d.slice(8, 10)}.${d.slice(5, 7)}.${d.slice(0, 4)}`
const tone = (v) => (v < 0 ? 'text-rose-400' : 'text-emerald-400')
const chip = (k) => badgeOf(k, props.badges)
</script>

<template>
  <section class="space-y-2" data-testid="cash-orphans">
    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Наличные без операции</div>
    <p class="text-[11px] text-slate-500">Их операция не видна в истории (счёт не выбран или операции нет в кеше). Записи по-прежнему считаются в итогах и смете.</p>
    <div class="border border-slate-800 rounded bg-slate-900/60 divide-y divide-slate-800">
      <div v-for="e in entries" :key="e.id" class="flex items-center gap-3 px-3 py-2" data-testid="orphan-row">
        <span class="text-xs text-slate-400 tabular-nums w-24 shrink-0">{{ dmy(e.day) }}</span>
        <span v-if="chip(e.badge)" class="px-1.5 py-0.5 rounded border text-[10px] font-bold shrink-0" :class="chip(e.badge).cls">{{ chip(e.badge).label }}</span>
        <span class="text-sm text-slate-200 truncate flex-1 min-w-0">{{ e.note || 'Наличные' }}</span>
        <span class="text-sm font-bold tabular-nums shrink-0" :class="tone(entrySigned(e, badges))">{{ entrySigned(e, badges) > 0 ? '+' : '−' }}{{ formatMoney(e.amount) }}</span>
        <span v-if="armedId === e.id" class="text-[10px] font-bold uppercase tracking-wider text-rose-300 shrink-0">Удалить?</span>
        <UiIconButton icon="trash" tone="danger" size="sm" :title="armedId === e.id ? 'Нажми ещё раз — удалить' : 'Удалить запись'" data-testid="orphan-remove" @click="onTrash(e)" />
      </div>
    </div>
  </section>
</template>

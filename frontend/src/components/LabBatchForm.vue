<script setup>
import { ref, computed, watch } from 'vue'
import UiButton from './ui/UiButton.vue'
import { cleanBatch, batchCost, batchCode, cardPerGram, cardGrams, batchUsage } from '../utils/labLedger.js'
import { formatMoney, toLocalDateStr } from '../utils/formatters.js'
import { numText } from '../utils/numfmt.js'

// Форма партии: дата, номер (ГГММДД-N подставляется сам), штуки и строки «карточка + граммы». Остаток карточки считается без этой партии.
const props = defineProps({
  batch: { type: Object, default: null },
  cards: { type: Array, default: () => [] },
  batches: { type: Array, default: () => [] },
  busy: { type: Boolean, default: false }
})
const emit = defineEmits(['save', 'cancel'])

const blank = () => ({ day: toLocalDateStr(new Date()), code: '', pieces: '', note: '', lines: [{ card_id: '', grams: '' }] })
const f = ref(blank())
const codeTouched = ref(false)
watch(() => props.batch, (b) => {
  codeTouched.value = !!b
  f.value = b
    ? { day: b.day, code: b.code, pieces: b.pieces ? String(b.pieces) : '', note: b.note || '', lines: b.lines.map((l) => ({ card_id: l.card_id, grams: String(l.grams) })) }
    : blank()
}, { immediate: true })
// номер следует за датой, пока его не правили руками
watch(() => [f.value.day, props.batches.length], () => {
  if (!codeTouched.value) f.value.code = batchCode(f.value.day, props.batches)
}, { immediate: true })

const used = computed(() => batchUsage(props.batches, props.batch?.id || ''))
const options = computed(() => [...props.cards]
  .sort((a, b) => a.substance.localeCompare(b.substance, 'ru') || (a.day < b.day ? -1 : 1))
  .map((c) => ({ card: c, left: Math.max(0, cardGrams(c) - (used.value.get(c.id) || 0)) })))
const optionOf = (id) => options.value.find((o) => o.card.id === id) || null
const label = (o) => `${o.card.substance} · ${o.card.maker || 'без производителя'} · ${numText(o.card.net_g, 3)} г · ост. ${numText(o.left, 3)} г · ${numText(cardPerGram(o.card), 4)} ₴/г`

const pieces = computed(() => Number(f.value.pieces) || 0)
const clean = computed(() => cleanBatch(f.value))
const cost = computed(() => (clean.value ? batchCost(clean.value, props.cards) : null))
// граммы одной карточки по всем строкам формы не должны превышать её остаток
const over = computed(() => {
  const sum = new Map()
  for (const l of f.value.lines) sum.set(l.card_id, (sum.get(l.card_id) || 0) + (Number(String(l.grams).replace(',', '.')) || 0))
  return [...sum].filter(([id, g]) => id && optionOf(id) && g > optionOf(id).left + 1e-6).map(([id]) => optionOf(id).card.substance)
})
const taken = (id, i) => f.value.lines.some((l, j) => j !== i && l.card_id === id)
const can = computed(() => !!clean.value && !over.value.length && !props.busy)

const addLine = () => f.value.lines.push({ card_id: '', grams: '' })
const dropLine = (i) => { if (f.value.lines.length > 1) f.value.lines.splice(i, 1) }
// значение числового поля — с точкой, не с запятой: иначе браузер его отбросит
const perPiece = (l) => { const g = Number(String(l.grams).replace(',', '.')); return pieces.value && g > 0 ? String(Math.round((g / pieces.value) * 1000) / 1000) : '' }
const setPer = (l, v) => { const n = Number(String(v).replace(',', '.')); l.grams = n > 0 && pieces.value ? String(Math.round(n * pieces.value * 1000) / 1000) : '' }

const submit = () => { if (can.value) emit('save', { ...(props.batch ? { id: props.batch.id } : {}), ...clean.value }) }
</script>

<template>
  <div class="space-y-3 rounded-md border border-slate-700/60 bg-slate-900/70 p-3 select-none" data-testid="batch-form">
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
      <label class="space-y-1">
        <span class="ui-label">Дата</span>
        <input v-model="f.day" type="date" class="ui-input" data-field="day" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Номер</span>
        <input v-model="f.code" type="text" class="ui-input tabular-nums" maxlength="24" data-field="code" @input="codeTouched = true" />
      </label>
      <label class="space-y-1">
        <span class="ui-label">Штук <span class="text-slate-500 normal-case">(необяз.)</span></span>
        <input v-model="f.pieces" type="number" min="0" step="1" class="ui-input tabular-nums" data-field="pieces" />
      </label>
      <label class="space-y-1 col-span-2 sm:col-span-1">
        <span class="ui-label">Пометка</span>
        <input v-model="f.note" type="text" class="ui-input" maxlength="200" data-field="note" />
      </label>
    </div>

    <div class="space-y-1.5">
      <div class="ui-label">Состав</div>
      <div v-for="(l, i) in f.lines" :key="i" class="grid grid-cols-[1fr_5.5rem_5.5rem_1.5rem] gap-2 items-center" data-testid="batch-line">
        <select v-model="l.card_id" class="ui-input" data-field="card">
          <option value="" disabled>Карточка товара…</option>
          <option v-for="o in options" :key="o.card.id" :value="o.card.id" :disabled="o.left <= 0 || taken(o.card.id, i)">{{ label(o) }}</option>
        </select>
        <input v-model="l.grams" type="number" min="0" step="any" class="ui-input tabular-nums" data-field="grams" placeholder="г всего" />
        <input :value="perPiece(l)" type="number" min="0" step="any" class="ui-input tabular-nums" data-field="per" placeholder="г/шт" :disabled="!pieces" @input="setPer(l, $event.target.value)" />
        <button type="button" class="text-slate-500 hover:text-rose-400 cursor-pointer disabled:opacity-30" :disabled="f.lines.length < 2" title="Убрать строку" @click="dropLine(i)">✕</button>
      </div>
      <button type="button" class="text-[11px] font-bold text-slate-500 hover:text-amber-300 transition-colors cursor-pointer" data-testid="batch-add-line" @click="addLine">+ Ещё вещество или карточка</button>
      <p v-if="!options.length" class="text-[11px] text-amber-300">Карточек пока нет: внеси их в «Учёте» под операциями «Расход».</p>
      <p v-if="over.length" class="text-[11px] text-rose-300" data-testid="batch-over">Больше остатка карточки: {{ over.join(', ') }}. Раздели вещество на две карточки.</p>
    </div>

    <div class="flex flex-wrap items-center justify-end gap-3">
      <span v-if="cost" class="text-[11px] text-slate-400 mr-auto tabular-nums" data-testid="batch-cost">
        Себестоимость <span class="text-slate-200 font-bold">{{ formatMoney(cost.cost) }}</span>
        <template v-if="cost.perPiece !== null"> · штука <span class="text-emerald-400 font-bold">{{ formatMoney(cost.perPiece) }}</span> · {{ numText(cost.gramsPerPiece, 3) }} г/шт</template>
      </span>
      <UiButton variant="cancel" size="sm" @click="emit('cancel')">Отмена</UiButton>
      <UiButton variant="primary" size="sm" :disabled="!can" @click="submit">{{ busy ? 'Сохраняю…' : batch ? 'Сохранить' : 'Создать партию' }}</UiButton>
    </div>
  </div>
</template>

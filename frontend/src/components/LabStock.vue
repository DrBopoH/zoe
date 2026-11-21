<script setup>
import { computed, ref } from 'vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiEmpty from './ui/UiEmpty.vue'
import { stockModel, cardPerGram, writeoffAmount } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'
import { numText, dmy } from '../utils/numfmt.js'

// Склад ХимЛаба: вещества и их карточки (конкретные закупки со своей ценой) с запасом от номинала до расчётного.
// Карточки вносятся в «Учёте» под операциями «Расход»; расход со склада считается из партий и списаний (used); «Списать остаток» переводит пыль карточки в убыток лаба.
const props = defineProps({
  cards: { type: Array, default: () => [] },
  used: { type: Object, default: () => new Map() }, // Map(id карточки → израсходованные граммы), считается из партий и списаний
  // списания остатка в убыток: документы { id, card_id, grams, day }; write(карточка, граммы) → true/false, unwrite(id)
  writeoffs: { type: Array, default: () => [] },
  write: { type: Function, default: null },
  unwrite: { type: Function, default: null }
})

const model = computed(() => stockModel(props.cards, props.used))
const confirming = ref('') // id карточки, у которой спрашиваем «Списать остаток?»
const busy = ref(false)
const armed = ref('')
let timer
const doWrite = async (r) => {
  busy.value = true
  try { if (await props.write(r.card, r.left)) confirming.value = '' } finally { busy.value = false }
}
const trash = (w) => {
  if (armed.value === w.id) { clearTimeout(timer); armed.value = ''; props.unwrite(w.id); return }
  armed.value = w.id
  clearTimeout(timer)
  timer = setTimeout(() => { armed.value = '' }, 4000)
}
const writtenOf = (c) => props.writeoffs.filter((w) => w.card_id === c.id)
const grams = (g) => `${numText(g, 3)} г`
const barCls = (p) => (p > 0.5 ? 'bg-emerald-400/80' : p > 0.2 ? 'bg-amber-400/80' : 'bg-rose-400/80')
</script>

<template>
  <div class="space-y-4 font-mono select-none" data-testid="lab-stock">
    <div class="flex flex-wrap gap-2" data-testid="stock-totals">
      <div class="px-3 py-2 rounded border border-slate-800 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-slate-400">Веществ</div>
        <div class="text-sm font-bold tabular-nums text-slate-200">{{ model.groups.length }}</div>
        <div class="text-[10px] text-slate-500">{{ model.cards }} карточ.</div>
      </div>
      <div class="px-3 py-2 rounded border border-slate-800 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-slate-400">Закуплено</div>
        <div class="text-sm font-bold tabular-nums text-slate-200">{{ formatMoney(model.valueTotal) }}</div>
      </div>
      <div class="px-3 py-2 rounded border border-emerald-500/30 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-emerald-300">На складе</div>
        <div class="text-sm font-bold tabular-nums text-emerald-400">{{ formatMoney(model.valueLeft) }}</div>
      </div>
    </div>

    <UiEmpty v-if="!model.groups.length">Карточек пока нет. Раскрой в «Учёте» операцию с бейджем «Расход» и нажми «+ Товар»: карточка попадёт на склад.</UiEmpty>

    <section v-for="g in model.groups" :key="g.key" class="rounded border border-slate-800 bg-slate-900/60" data-testid="stock-group">
      <header class="flex flex-wrap items-baseline justify-between gap-x-4 px-3 py-2 border-b border-slate-800">
        <div class="text-sm font-bold text-slate-200">{{ g.name }}</div>
        <div class="text-[11px] text-slate-400 tabular-nums">
          <span class="text-slate-200 font-bold">{{ grams(g.left) }}</span> из {{ grams(g.nominal) }}
          · <span class="text-emerald-400 font-bold">{{ formatMoney(g.valueLeft) }}</span> из {{ formatMoney(g.valueTotal) }}
        </div>
      </header>
      <ul>
        <li v-for="r in g.rows" :key="r.card.id" class="px-3 py-2 border-b border-slate-800/60 last:border-b-0 space-y-1.5" data-testid="stock-card">
          <div class="flex flex-wrap items-baseline justify-between gap-x-4 text-[11px]">
            <div class="text-slate-300">
              {{ r.card.maker || 'без производителя' }} · {{ numText(r.card.net_g, 3) }} г
              <span class="text-slate-500">· {{ r.card.qty }} × {{ formatMoney(r.card.price) }}</span>
              <span v-if="r.card.note" class="text-slate-500"> · {{ r.card.note }}</span>
            </div>
            <div class="text-slate-500 tabular-nums">{{ dmy(r.card.day) }} · {{ numText(cardPerGram(r.card), 4) }} ₴/г</div>
          </div>
          <div class="flex items-center gap-3">
            <div class="flex-1 h-1.5 rounded bg-slate-800 overflow-hidden" role="img" :aria-label="`Осталось ${Math.round(r.pct * 100)} %`">
              <div class="h-full rounded" :class="barCls(r.pct)" :style="{ width: `${Math.round(r.pct * 100)}%` }" data-testid="stock-bar" />
            </div>
            <div class="text-[11px] text-slate-400 tabular-nums whitespace-nowrap">{{ grams(r.left) }} из {{ grams(r.nominal) }} · {{ formatMoney(r.valueLeft) }}</div>
          </div>
          <div v-for="w in writtenOf(r.card)" :key="w.id" class="group flex flex-wrap items-center gap-x-3 text-[11px]" data-testid="writeoff-row">
            <span class="text-[9px] px-1.5 py-0.5 rounded border font-bold bg-rose-500/15 text-rose-300 border-rose-500/30">Списано</span>
            <span class="text-slate-500">{{ dmy(w.day) }}</span>
            <span class="text-slate-300 tabular-nums">{{ grams(w.grams) }} в убыток</span>
            <span class="ml-auto text-rose-300 tabular-nums">{{ formatMoney(writeoffAmount(w, r.card)) }}</span>
            <span v-if="unwrite" class="inline-flex items-center gap-0.5 transition-opacity" :class="armed === w.id ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'">
              <span v-if="armed === w.id" class="text-[10px] font-bold uppercase tracking-wider text-rose-300 mr-1">Вернуть?</span>
              <UiIconButton icon="trash" tone="danger" size="sm" :title="armed === w.id ? 'Нажми ещё раз — вернуть остаток на склад' : 'Отменить списание'" data-testid="writeoff-remove" @click.stop="trash(w)" />
            </span>
          </div>
          <div v-if="write && r.left > 0 && r.valueLeft > 0" class="text-[11px]">
            <span v-if="confirming === r.card.id" class="inline-flex flex-wrap items-center gap-2" data-testid="writeoff-confirm">
              <span class="text-slate-300">Списать {{ grams(r.left) }} ({{ formatMoney(r.valueLeft) }} ₴) в убыток?</span>
              <button type="button" class="font-bold text-rose-300 hover:text-rose-200 cursor-pointer disabled:opacity-40" :disabled="busy" data-testid="writeoff-yes" @click.stop="doWrite(r)">Списать</button>
              <button type="button" class="text-slate-500 hover:text-slate-300 cursor-pointer" @click.stop="confirming = ''">Отмена</button>
            </span>
            <button v-else type="button" class="font-bold text-slate-500 hover:text-rose-300 transition-colors cursor-pointer" data-testid="writeoff-start" @click.stop="confirming = r.card.id">Списать остаток</button>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

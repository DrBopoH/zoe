<script setup>
import { computed } from 'vue'
import { portfolioStats } from '../utils/capital.js'
import { currentInflation, signedPct, plainPct } from '../utils/macro.js'
import { money } from '../utils/formatters.js'
import { bondDollarYield } from '../utils/dollar.js'
import { formatDateLong } from '../utils/formatters.js'

// Портфель ОВДП: средняя ставка и срок по стоимости позиций, реальная доходность (ставка с поправкой на инфляцию г/г), ближайшая выплата.
const props = defineProps({
  positions: { type: Array, default: () => [] },
  macro: { type: Object, default: () => ({}) }, // макро-ряды: инфляция для реальной доходности
  today: { type: String, required: true },
  assume: { type: Object, default: null } // допущения для долларовой доходности (dollarAssumptions)
})
const inflation = computed(() => currentInflation(props.macro))
const st = computed(() => portfolioStats(props.positions, props.today, inflation.value))
const usdy = computed(() => (st.value && props.assume ? bondDollarYield({ rate: st.value.rate, currency: 'UAH', ...props.assume }) : { usd: null, realUsd: null }))
const yearsText = (y) => `${String(y).replace('.', ',')} г.`
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-5 flex flex-col gap-3">
    <div class="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1">
      <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Портфель ОВДП</div>
      <div v-if="st && st.next" class="text-xs text-slate-400">Ближайшая выплата <b class="text-slate-200">{{ formatDateLong(st.next.date) }}</b> · <b class="text-emerald-400 tabular-nums">+{{ money(st.next.amount) }}</b></div>
    </div>
    <div v-if="!st" class="text-xs text-slate-600 uppercase tracking-wider py-4 text-center">Нет облигаций со ставкой и погашением в справочнике ICU</div>
    <template v-else>
      <div class="grid gap-3 grid-cols-2 sm:grid-cols-3 auto-rows-fr flex-1 text-xs">
        <div class="bg-slate-950/40 rounded p-3" :title="`Стоимость позиций по цене покупки`">
          <div class="text-slate-500">В облигациях</div>
          <div class="text-lg font-bold text-slate-100 tabular-nums">{{ money(st.value) }}</div>
        </div>
        <div class="bg-slate-950/40 rounded p-3" title="Средняя купонная ставка, взвешенная по стоимости позиций (доходность к погашению по цене покупки может отличаться)">
          <div class="text-slate-500">Средняя ставка</div>
          <div class="text-lg font-bold text-amber-300 tabular-nums">{{ plainPct(st.rate) }}</div>
        </div>
        <div class="bg-slate-950/40 rounded p-3" :title="st.real === null ? 'Нет данных об инфляции: нажми «Обновить» на экране «Инфляция»' : `(1 + ставка) / (1 + инфляция) − 1, инфляция г/г ${plainPct(inflation)}`">
          <div class="text-slate-500">Реальная доходность</div>
          <div class="text-lg font-bold tabular-nums" :class="st.real === null ? 'text-slate-600' : st.real >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(st.real) }}</div>
        </div>
        <div v-if="usdy.usd !== null" class="bg-slate-950/40 rounded p-3" :title="`(1 + ставка) / (1 + рост доллара) − 1; доллар дорожал на ${plainPct(assume.devUsd)} в год за ${assume.years} лет`">
          <div class="text-slate-500">Доходность в $</div>
          <div class="text-lg font-bold tabular-nums" :class="usdy.usd >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(usdy.usd) }}</div>
        </div>
        <div v-if="usdy.realUsd !== null" class="bg-slate-950/40 rounded p-3" :title="`Доходность в $ с поправкой на инфляцию США ${plainPct(assume.usInflation)} г/г`">
          <div class="text-slate-500">Реальная в $</div>
          <div class="text-lg font-bold tabular-nums" :class="usdy.realUsd >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(usdy.realUsd) }}</div>
        </div>
        <div class="bg-slate-950/40 rounded p-3" title="Средний срок до погашения, взвешенный по стоимости">
          <div class="text-slate-500">Срок до погашения</div>
          <div class="text-lg font-bold text-slate-100 tabular-nums">{{ yearsText(st.years) }}</div>
        </div>
      </div>
      <div v-if="st.skipped" class="text-[11px] text-slate-600">Не учтено позиций: {{ st.skipped }} (нет ставки или погашения в справочнике)</div>
    </template>
  </section>
</template>

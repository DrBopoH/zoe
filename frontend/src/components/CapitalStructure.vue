<script setup>
import { computed } from 'vue'
import { capitalStructure } from '../utils/capital.js'
import { money } from '../utils/formatters.js'

// Из чего состоит капитал: полоса по слоям (карты, деньги на брокерском, ОВДП, крипто) и подписи с суммами и долями.
const props = defineProps({
  accounts: { type: Array, default: () => [] },
  rates: { type: Object, default: null } // { usdUah, eurUah }: с курсом всё считается в долларах
})
const st = computed(() => capitalStructure(props.accounts, props.rates))
const val = (v) => money(v, st.value.unit)
const pctText = (v) => (v === null ? '' : `${String(v).replace('.', ',')} %`)
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-3">
    <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Структура капитала</div>
    <div v-if="!st.parts.length" class="text-xs text-slate-600 uppercase tracking-wider py-4 text-center">Нет данных по счетам</div>
    <template v-else>
      <div class="flex h-3 rounded overflow-hidden bg-slate-800" role="img" :aria-label="`Структура капитала: ${st.parts.map((p) => p.label + ' ' + pctText(p.pct)).join(', ')}`">
        <div v-for="p in st.parts.filter((x) => x.value > 0)" :key="p.key" :style="{ width: `${(p.value / st.parts.filter((x) => x.value > 0).reduce((s, x) => s + x.value, 0)) * 100}%`, backgroundColor: p.color }" :title="`${p.label}: ${val(p.value)}`"></div>
      </div>
      <ul class="space-y-1.5 text-xs">
        <li v-for="p in st.parts" :key="p.key" class="flex items-center gap-2" :title="p.hint">
          <span class="inline-block w-2.5 h-2.5 rounded-sm shrink-0" :style="{ backgroundColor: p.color }"></span>
          <span class="text-slate-300">{{ p.label }}</span>
          <span class="ml-auto tabular-nums text-slate-200 font-bold">{{ val(p.value) }}</span>
          <span class="w-14 text-right tabular-nums text-slate-500">{{ pctText(p.pct) }}</span>
        </li>
      </ul>
      <div v-if="st.currencies.length > 1" class="pt-2 border-t border-slate-800 space-y-1.5">
        <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">По валюте</div>
        <div class="flex h-2 rounded overflow-hidden bg-slate-800">
          <div v-for="c in st.currencies" :key="c.key" :style="{ width: `${c.pct}%`, backgroundColor: c.color }" :title="`${c.label}: ${val(c.value)}`"></div>
        </div>
        <ul class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
          <li v-for="c in st.currencies" :key="c.key" class="flex items-center gap-1.5 text-slate-400">
            <span class="inline-block w-2 h-2 rounded-sm" :style="{ backgroundColor: c.color }"></span>{{ c.label }} <b class="text-slate-200 tabular-nums">{{ pctText(c.pct) }}</b>
          </li>
        </ul>
      </div>
      <div v-if="st.skipped" class="text-[11px] text-slate-600">Не учтено счетов: {{ st.skipped }} (валюта без курса)</div>
    </template>
  </section>
</template>

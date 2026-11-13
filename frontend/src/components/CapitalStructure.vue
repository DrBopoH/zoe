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
const live = computed(() => st.value.parts.filter((x) => x.value > 0))
const val = (v) => money(v, st.value.unit)
const pctText = (v) => (v === null ? '' : `${String(v).replace('.', ',')} %`)
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-5 space-y-4">
    <div class="flex items-baseline justify-between gap-3">
      <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Структура капитала</div>
      <div v-if="st.skipped" class="text-[11px] text-slate-600">Не учтено счетов: {{ st.skipped }} (валюта без курса)</div>
    </div>
    <div v-if="!st.parts.length" class="text-xs text-slate-600 uppercase tracking-wider py-4 text-center">Нет данных по счетам</div>
    <template v-else>
      <div class="flex h-4 gap-0.5 rounded overflow-hidden" role="img" :aria-label="`Структура капитала: ${st.parts.map((p) => p.label + ' ' + pctText(p.pct)).join(', ')}`">
        <div v-for="p in live" :key="p.key" :style="{ flex: `${p.value} 1 0`, backgroundColor: p.color }" :title="`${p.label}: ${val(p.value)}`"></div>
      </div>
      <ul class="grid gap-3 grid-cols-2 lg:grid-cols-4">
        <li v-for="p in st.parts" :key="p.key" class="border-l-2 pl-3 space-y-0.5" :style="{ borderColor: p.color }" :title="p.hint">
          <div class="text-[11px] text-slate-500">{{ p.label }}</div>
          <div class="text-lg font-bold tabular-nums text-slate-100">{{ val(p.value) }}</div>
          <div class="text-xs tabular-nums text-slate-500">{{ pctText(p.pct) || '0 %' }}</div>
        </li>
      </ul>
      <div v-if="st.currencies.length > 1" class="flex flex-wrap items-center gap-x-5 gap-y-2 pt-3 border-t border-slate-800">
        <span class="text-[10px] uppercase tracking-wider font-bold text-slate-500">По валюте</span>
        <div class="flex h-1.5 gap-0.5 rounded overflow-hidden flex-1 min-w-32 max-w-md">
          <div v-for="c in st.currencies" :key="c.key" :style="{ flex: `${c.pct} 1 0`, backgroundColor: c.color }" :title="`${c.label}: ${val(c.value)}`"></div>
        </div>
        <ul class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
          <li v-for="c in st.currencies" :key="c.key" class="flex items-center gap-1.5 text-slate-400">
            <span class="inline-block w-2 h-2 rounded-sm" :style="{ backgroundColor: c.color }"></span>{{ c.label }} <b class="text-slate-200 tabular-nums">{{ pctText(c.pct) }}</b>
          </li>
        </ul>
      </div>
    </template>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { capitalStructure } from '../utils/capital.js'
import { money } from '../utils/formatters.js'

// Из чего состоит капитал: кольцо по слоям (карты, брокерский, ОВДП, крипто), внутреннее кольцо — по валюте, справа легенда с суммами и долями.
const props = defineProps({
  accounts: { type: Array, default: () => [] },
  rates: { type: Object, default: null } // { usdUah, eurUah }: с курсом всё считается в долларах
})
const st = computed(() => capitalStructure(props.accounts, props.rates))
const live = computed(() => st.value.parts.filter((x) => x.value > 0))
const val = (v) => money(v, st.value.unit)
const pctText = (v) => (v === null ? '' : `${String(v).replace('.', ',')} %`)

const CX = 70
const hot = ref(null) // ключ слоя или валюты под курсором: показываем его в центре кольца
// Кольцо из дуг: stroke-dasharray по окружности, зазор 1.5 пункта между дугами.
function ring(items, r) {
  const c = 2 * Math.PI * r
  const sum = items.reduce((s, x) => s + x.value, 0)
  if (!sum) return []
  let off = 0
  return items.map((x) => {
    const len = (x.value / sum) * c
    const gap = items.length > 1 ? Math.min(1.5, len / 2) : 0
    const arc = { ...x, dash: `${Math.max(len - gap, 0)} ${c}`, offset: -off }
    off += len
    return arc
  })
}
const outer = computed(() => ring(live.value, 58))
const inner = computed(() => ring(st.value.currencies, 41))
const focus = computed(() => live.value.find((p) => p.key === hot.value) || st.value.currencies.find((c) => c.key === hot.value) || null)
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-5 flex flex-col gap-3">
    <div class="flex items-baseline justify-between gap-3">
      <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Структура капитала</div>
      <div v-if="st.skipped" class="text-[11px] text-slate-600">Не учтено счетов: {{ st.skipped }} (валюта без курса)</div>
    </div>
    <div v-if="!st.parts.length" class="text-xs text-slate-600 uppercase tracking-wider py-4 text-center">Нет данных по счетам</div>
    <div v-else class="flex flex-wrap items-center gap-x-6 gap-y-4">
      <div class="relative shrink-0 w-36 h-36" @mouseleave="hot = null">
        <svg viewBox="0 0 140 140" class="w-full h-full -rotate-90" role="img" :aria-label="`Структура капитала: ${st.parts.map((p) => p.label + ' ' + pctText(p.pct)).join(', ')}`">
          <circle :cx="CX" :cy="CX" r="58" fill="none" stroke="currentColor" class="text-slate-800/60" stroke-width="14" />
          <circle v-for="a in outer" :key="a.key" :cx="CX" :cy="CX" r="58" fill="none" :stroke="a.color" stroke-width="14" :stroke-dasharray="a.dash" :stroke-dashoffset="a.offset" :opacity="hot && hot !== a.key ? 0.35 : 1" class="transition-opacity" @mouseenter="hot = a.key"><title>{{ a.label }}: {{ val(a.value) }}</title></circle>
          <circle v-for="a in inner" :key="a.key" :cx="CX" :cy="CX" r="41" fill="none" :stroke="a.color" stroke-width="6" :stroke-dasharray="a.dash" :stroke-dashoffset="a.offset" :opacity="hot && hot !== a.key ? 0.35 : 1" class="transition-opacity" @mouseenter="hot = a.key"><title>{{ a.label }}: {{ val(a.value) }}</title></circle>
        </svg>
        <div class="absolute inset-0 flex flex-col items-center justify-center text-center pointer-events-none px-8">
          <template v-if="focus">
            <div class="text-[10px] text-slate-500 leading-tight">{{ focus.label }}</div>
            <div class="text-base font-bold tabular-nums text-slate-100 leading-tight">{{ pctText(focus.pct) }}</div>
          </template>
          <template v-else>
            <div class="text-[10px] uppercase tracking-wider text-slate-500">Капитал</div>
          </template>
        </div>
      </div>
      <div class="flex-1 min-w-52 space-y-3">
        <ul class="space-y-1.5">
          <li v-for="p in st.parts" :key="p.key" class="flex items-center gap-2 text-xs rounded px-1.5 py-1 -mx-1.5 hover:bg-slate-800/50" :title="p.hint" @mouseenter="hot = p.key" @mouseleave="hot = null">
            <span class="inline-block w-2 h-2 rounded-sm shrink-0" :style="{ backgroundColor: p.color }"></span>
            <span class="text-slate-400 flex-1 truncate">{{ p.label }}</span>
            <b class="text-slate-100 tabular-nums">{{ val(p.value) }}</b>
            <span class="text-slate-500 tabular-nums w-12 text-right">{{ pctText(p.pct) || '0 %' }}</span>
          </li>
        </ul>
        <ul v-if="st.currencies.length > 1" class="space-y-1.5 pt-3 border-t border-slate-800">
          <li v-for="c in st.currencies" :key="c.key" class="flex items-center gap-2 text-xs rounded px-1.5 py-1 -mx-1.5 hover:bg-slate-800/50" @mouseenter="hot = c.key" @mouseleave="hot = null">
            <span class="inline-block w-2 h-2 rounded-full shrink-0" :style="{ backgroundColor: c.color }"></span>
            <span class="text-slate-400 flex-1">{{ c.label }}</span>
            <b class="text-slate-100 tabular-nums">{{ val(c.value) }}</b>
            <span class="text-slate-500 tabular-nums w-12 text-right">{{ pctText(c.pct) }}</span>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

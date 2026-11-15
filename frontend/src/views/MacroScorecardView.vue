<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { hasPoints, signedPct, plainPct, unitText } from '../utils/macro.js'
import { stressAt, stressSeries, stressLevel, STRESS_TEXT, signalRows, signalTally, regime, realRateRows } from '../utils/scorecard.js'

// Экран «Скоркард»: сводка из уже загруженных рядов — режим экономики по правилам, индекс стресса, светофор опережающих сигналов
// и реальные ставки стран. Это сводка, а не прогноз и не торговый сигнал: у каждого вывода видно, из чего он получен.
const props = defineProps({
  econ: { type: Object, default: () => ({}) }, // ряды США (источники usecon, uscredit, usjobs, usprices, us) одним запросом
  countries: { type: Array, default: () => [] },
  cseries: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'usecon')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся ряды экономики США; кредит и труд — в своих разделах'

const has = computed(() => hasPoints(props.econ))
const reg = computed(() => regime(props.econ, props.today))
const lastDate = (pts) => (pts && pts.length ? pts[pts.length - 1].d : null)
const stress = computed(() => stressAt(props.econ, lastDate(props.econ.us_vix) || props.today))
const level = computed(() => stressLevel(stress.value?.value))
const stressSer = computed(() => ({ stress_idx: stressSeries(props.econ) }))
const signals = computed(() => signalRows(props.econ, props.today))
const tally = computed(() => signalTally(signals.value))
const rates = computed(() => realRateRows(props.countries, props.cseries, props.today))

const STATE = { ok: ['text-emerald-400', 'спокойно'], warn: ['text-amber-300', 'внимание'], alarm: ['text-rose-400', 'тревога'] }
const LEVEL_TONE = { calm: 'text-emerald-400', normal: 'text-slate-200', tense: 'text-amber-300', stress: 'text-rose-400' }
const REG_TONE = { recession: 'text-rose-400', stagflation: 'text-rose-400', overheating: 'text-amber-300', disinflation: 'text-sky-300', goldilocks: 'text-emerald-400' }
const sval = (r) => (r.unit === '%' ? plainPct(r.value) : unitText(r.value, r.unit.split(' ')[0]))
const z = (v) => (v > 0 ? '+' : v < 0 ? '−' : '') + String(Math.abs(v)).replace('.', ',')
const tone = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Скоркард" :running="running" @refresh="refresh">
      <span class="text-slate-500">сводка по данным, не прогноз и не сигнал к сделке</span>
    </MarketHeader>

    <div v-if="!has" class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-500">{{ EMPTY }}</div>

    <template v-else>
      <section class="grid gap-3 md:grid-cols-2">
        <div class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs">
          <div class="mb-1 text-slate-500">Режим экономики США</div>
          <template v-if="reg">
            <div class="text-lg font-bold" :class="REG_TONE[reg.key]">{{ reg.label }}</div>
            <div class="text-slate-400">{{ reg.note }}</div>
            <ul class="mt-2 list-disc space-y-0.5 pl-4 text-slate-500"><li v-for="w in reg.why" :key="w">{{ w }}</li></ul>
          </template>
          <div v-else class="text-slate-500">Нужны рост ВВП и инфляция США.</div>
        </div>
        <div class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs">
          <div class="mb-1 text-slate-500">Индекс стресса (среднее z-оценок, 0 — обычно)</div>
          <template v-if="stress">
            <div class="text-lg font-bold tabular-nums" :class="LEVEL_TONE[level]">{{ z(stress.value) }} <span class="text-sm font-normal">{{ STRESS_TEXT[level] }}</span></div>
            <table class="mt-2 w-full tabular-nums">
              <tr v-for="p in stress.parts" :key="p.id" class="text-slate-400"><td class="py-0.5">{{ p.label }}</td><td class="text-right" :class="p.z > 1 ? 'text-rose-400' : p.z < -0.5 ? 'text-emerald-400' : 'text-slate-300'">{{ z(p.z) }}</td></tr>
            </table>
          </template>
          <div v-else class="text-slate-500">Нужно хотя бы три компонента (VIX, спред HY, NFCI, кривая, доллар): загрузи «Кредит и доллар».</div>
        </div>
      </section>

      <MacroPanel title="Индекс стресса США" unit="σ" :series="stressSer" :today="today" :ids="['stress_idx']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Ноль — обычный фон, выше — напряжение на рынках">
        <template #summary><span class="text-slate-500">z-оценка каждого компонента против его истории, затем среднее</span></template>
      </MacroPanel>

      <section class="rounded-lg border border-slate-800 bg-slate-900/40 text-xs overflow-x-auto">
        <div class="flex flex-wrap items-center gap-x-4 px-3 pt-3 text-slate-400">
          <b class="text-slate-200">Опережающие сигналы</b>
          <span class="text-emerald-400">спокойно: {{ tally.ok }}</span><span class="text-amber-300">внимание: {{ tally.warn }}</span><span class="text-rose-400">тревога: {{ tally.alarm }}</span>
        </div>
        <table class="mt-2 w-full tabular-nums">
          <tbody>
            <tr v-for="r in signals" :key="r.key" class="border-t border-slate-800/60">
              <td class="px-3 py-1.5 text-slate-300">{{ r.label }}</td>
              <td class="px-3 py-1.5 text-right text-slate-200">{{ sval(r) }}</td>
              <td class="px-3 py-1.5 font-bold" :class="STATE[r.state][0]">{{ STATE[r.state][1] }}</td>
              <td class="hidden px-3 py-1.5 text-slate-500 md:table-cell">{{ r.hint }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>

    <section class="rounded-lg border border-slate-800 bg-slate-900/40 text-xs overflow-x-auto">
      <div class="px-3 pt-3 text-slate-400"><b class="text-slate-200">Реальные ставки и кэрри по странам</b> — ставка центробанка минус инфляция; кэрри — ставка минус ставка ФРС</div>
      <table v-if="rates.length" class="mt-2 w-full tabular-nums">
        <thead><tr class="text-slate-500"><th class="px-3 py-1 text-left font-normal">Страна</th><th class="px-3 py-1 text-right font-normal">Ставка</th><th class="px-3 py-1 text-right font-normal">Инфляция</th><th class="px-3 py-1 text-right font-normal">Реальная</th><th class="px-3 py-1 text-right font-normal">Кэрри к США</th></tr></thead>
        <tbody>
          <tr v-for="r in rates" :key="r.cc" class="border-t border-slate-800/60">
            <td class="px-3 py-1.5 text-slate-300">{{ r.name }}</td>
            <td class="px-3 py-1.5 text-right text-slate-300">{{ plainPct(r.policy) }}</td>
            <td class="px-3 py-1.5 text-right text-slate-300">{{ plainPct(r.cpi) }}</td>
            <td class="px-3 py-1.5 text-right font-bold" :class="tone(r.real)">{{ signedPct(r.real) }}</td>
            <td class="px-3 py-1.5 text-right" :class="tone(r.carry)">{{ r.carry === null ? '—' : signedPct(r.carry) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="p-3 text-slate-500">Нет данных по странам. Открой «Страны» или «Курсы» и нажми «Обновить».</div>
    </section>
  </div>
</template>

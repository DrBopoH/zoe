<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, hasPoints } from '../utils/macro.js'

// Экран «Реальные ставки»: что рынок платит сверх инфляции (TIPS), какую инфляцию он закладывает (брейкивен), срочная премия и ипотека.
const props = defineProps({
  real: { type: Object, default: () => ({}) }, // us_real5/10, us_be5/10, us_fwd5y5y, us_termprem, us_mortgage
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'real')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся реальные ставки и ожидаемая инфляция США из FRED'
const real10 = computed(() => lastPoint(props.real.us_real10))
const be10 = computed(() => lastPoint(props.real.us_be10))
const fwd = computed(() => lastPoint(props.real.us_fwd5y5y))
const prem = computed(() => lastPoint(props.real.us_termprem))
const mortgage = computed(() => lastPoint(props.real.us_mortgage))
const has = computed(() => hasPoints(props.real))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Реальные ставки США" :running="running" @refresh="refresh" />

    <MacroPanel title="Реальная доходность Treasury (TIPS)" :series="real" :today="today" :ids="['us_real5', 'us_real10']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Реальная ставка — доходность облигации за вычетом ожидаемой инфляции">
      <template #summary>
        <span v-if="real10" class="text-slate-400">10 лет <b class="tabular-nums" :class="real10.v < 0 ? 'text-rose-400' : 'text-emerald-400'">{{ plainPct(real10.v) }}</b><span class="text-slate-600"> · {{ real10.v < 0 ? 'деньги дешевле инфляции' : 'деньги дороже инфляции' }}</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Ожидаемая инфляция: брейкивен и 5y5y" :series="real" :today="today" :ids="['us_be5', 'us_be10', 'us_fwd5y5y']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Брейкивен — инфляция, при которой обычная облигация и TIPS приносят одинаково">
      <template #summary>
        <span v-if="be10" class="text-slate-400">на 10 лет <b class="tabular-nums text-amber-300">{{ plainPct(be10.v) }}</b></span>
        <span v-if="fwd" class="text-slate-400">5y5y <b class="tabular-nums text-violet-300">{{ plainPct(fwd.v) }}</b><span class="text-slate-600"> · ожидание через 5 лет на следующие 5</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Срочная премия 10-летних Treasury" unit="п. п." :series="real" :today="today" :ids="['us_termprem']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Модель ФРБ Нью-Йорка: плата за риск держать длинные облигации">
      <template #summary>
        <span v-if="prem" class="text-slate-400">Сейчас <b class="tabular-nums text-pink-300">{{ plainPct(prem.v) }} п. п.</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Ставка 30-летней ипотеки (Freddie Mac)" :series="real" :today="today" :ids="['us_mortgage']" :empty="EMPTY">
      <template #summary>
        <span v-if="mortgage" class="text-slate-400">Сейчас <b class="tabular-nums text-emerald-400">{{ plainPct(mortgage.v) }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса). Подсказки — как эти показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

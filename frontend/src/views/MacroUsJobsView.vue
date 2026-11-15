<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, unitText, realWageGrowth, hasPoints } from '../utils/macro.js'

// Экран «Рынок труда США»: занятость, заявки на пособие, вакансии, безработица и зарплаты против инфляции (FRED).
const props = defineProps({
  jobs: { type: Object, default: () => ({}) }, // us_nfp, us_claims, us_jolts, us_wages_yoy, us_eci_yoy, us_u6, us_unrate, us_cpi_yoy
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'jobs')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянется рынок труда США из FRED'
const monthOf = (p) => (p ? `${p.d.slice(5, 7)}.${p.d.slice(0, 4)}` : '')
const nfp = computed(() => lastPoint(props.jobs.us_nfp))
const claims = computed(() => lastPoint(props.jobs.us_claims))
const jolts = computed(() => lastPoint(props.jobs.us_jolts))
const u6 = computed(() => lastPoint(props.jobs.us_u6))
const unrate = computed(() => lastPoint(props.jobs.us_unrate))
const wages = computed(() => lastPoint(props.jobs.us_wages_yoy))
const real = computed(() => realWageGrowth(props.jobs.us_wages_yoy, props.jobs.us_cpi_yoy))
const num = (v) => String(Math.round(v * 10) / 10).replace('.', ',')
const has = computed(() => hasPoints(props.jobs))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Рынок труда США" :running="running" @refresh="refresh">
      <span v-if="nfp" class="text-slate-500">занятость: последняя публикация {{ monthOf(nfp) }}</span>
    </MarketHeader>

    <MacroPanel title="Прирост занятых за месяц (вне сельского хозяйства)" unit="тыс." :series="jobs" :today="today" :ids="['us_nfp']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Отрицательный месяц: рабочих мест стало меньше">
      <template #summary>
        <span v-if="nfp" class="text-slate-400">{{ monthOf(nfp) }} <b class="tabular-nums" :class="nfp.v >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ nfp.v > 0 ? '+' : '' }}{{ unitText(nfp.v, 'тыс.') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Безработица: официальная и U-6 (с неполной занятостью)" :series="jobs" :today="today" :ids="['us_unrate', 'us_u6']" :empty="EMPTY">
      <template #summary>
        <span v-if="unrate" class="text-slate-400">официальная <b class="tabular-nums text-sky-300">{{ plainPct(unrate.v) }}</b></span>
        <span v-if="u6" class="text-slate-400">U-6 <b class="tabular-nums text-orange-300">{{ plainPct(u6.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Заявки на пособие по безработице (в неделю)" unit="чел." :series="jobs" :today="today" :ids="['us_claims']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Самый быстрый показатель рынка труда: растёт раньше остальных">
      <template #summary>
        <span v-if="claims" class="text-slate-400">неделя {{ claims.d.split('-').reverse().join('.') }} <b class="tabular-nums text-amber-300">{{ unitText(claims.v, 'чел.') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Открытые вакансии (JOLTS)" unit="тыс." :series="jobs" :today="today" :ids="['us_jolts']" :empty="EMPTY">
      <template #summary>
        <span v-if="jolts" class="text-slate-400">{{ monthOf(jolts) }} <b class="tabular-nums text-sky-300">{{ unitText(jolts.v, 'тыс.') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Зарплаты: средний заработок и стоимость рабочей силы" :series="jobs" :today="today" :ids="['us_wages_yoy', 'us_eci_yoy', 'us_cpi_yoy']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Зарплаты быстрее инфляции — реальный доход растёт">
      <template #summary>
        <span v-if="wages" class="text-slate-400">заработок <b class="tabular-nums text-emerald-400">{{ plainPct(wages.v) }}</b></span>
        <span v-if="real" class="text-slate-400">реально <b class="tabular-nums" :class="real.v >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ real.v > 0 ? '+' : '' }}{{ num(real.v) }} п. п.</b><span class="text-slate-600"> · {{ real.v >= 0 ? 'зарплаты обгоняют инфляцию' : 'инфляция обгоняет зарплаты' }}</span></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса). Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

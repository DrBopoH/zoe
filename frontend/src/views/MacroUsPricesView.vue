<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, hasPoints, FED_INFLATION_TARGET } from '../utils/macro.js'

// Экран «Цены США»: общая и базовая инфляция, PCE (по нему ФРС меряет цель), цены производителей и жильё (FRED).
const props = defineProps({
  prices: { type: Object, default: () => ({}) }, // us_cpi_yoy, us_corecpi_yoy/mom, us_pce_yoy, us_corepce_yoy, us_ppi_yoy, us_shelter_yoy
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'prices')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся цены США из FRED'
const monthOf = (p) => (p ? `${p.d.slice(5, 7)}.${p.d.slice(0, 4)}` : '')
const cpi = computed(() => lastPoint(props.prices.us_cpi_yoy))
const core = computed(() => lastPoint(props.prices.us_corecpi_yoy))
const corePce = computed(() => lastPoint(props.prices.us_corepce_yoy))
const coreMom = computed(() => lastPoint(props.prices.us_corecpi_mom))
const ppi = computed(() => lastPoint(props.prices.us_ppi_yoy))
const shelter = computed(() => lastPoint(props.prices.us_shelter_yoy))
const gap = computed(() => (corePce.value ? Math.round((corePce.value.v - FED_INFLATION_TARGET) * 100) / 100 : null))
const has = computed(() => hasPoints(props.prices))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Цены США" :running="running" @refresh="refresh">
      <span v-if="core" class="text-slate-500">последняя публикация {{ monthOf(core) }}</span>
    </MarketHeader>

    <MacroPanel title="Инфляция: общая, базовая и базовый PCE" :series="prices" :today="today" :ids="['us_cpi_yoy', 'us_corecpi_yoy', 'us_corepce_yoy']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Базовые меры без еды и энергии: цель ФРС в 2 % считается по базовому PCE">
      <template #summary>
        <span v-if="cpi" class="text-slate-400">CPI <b class="tabular-nums text-amber-300">{{ plainPct(cpi.v) }}</b></span>
        <span v-if="core" class="text-slate-400">базовый CPI <b class="tabular-nums text-orange-300">{{ plainPct(core.v) }}</b></span>
        <span v-if="corePce" class="text-slate-400">базовый PCE <b class="tabular-nums text-emerald-400">{{ plainPct(corePce.v) }}</b><span v-if="gap !== null" class="text-slate-600"> · {{ gap > 0 ? 'выше цели ФРС на ' + String(gap).replace('.', ',') + ' п. п.' : gap < 0 ? 'ниже цели' : 'на цели' }}</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Базовый CPI к прошлому месяцу" :series="prices" :today="today" :ids="['us_corecpi_mom']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Месячный темп: 0,2 % в месяц — это около 2,4 % в год">
      <template #summary>
        <span v-if="coreMom" class="text-slate-400">{{ monthOf(coreMom) }} <b class="tabular-nums text-orange-300">{{ plainPct(coreMom.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Цены производителей и жильё в CPI" :series="prices" :today="today" :ids="['us_ppi_yoy', 'us_shelter_yoy']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Цены производителей опережают потребительские, жильё — самая медленная и тяжёлая часть CPI">
      <template #summary>
        <span v-if="ppi" class="text-slate-400">производители <b class="tabular-nums text-pink-300">{{ plainPct(ppi.v) }}</b></span>
        <span v-if="shelter" class="text-slate-400">жильё <b class="tabular-nums text-sky-300">{{ plainPct(shelter.v) }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса), изменение к тому же месяцу прошлого года. Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

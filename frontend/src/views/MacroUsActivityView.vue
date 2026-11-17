<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, unitText, hasPoints } from '../utils/macro.js'

// Экран «Активность США»: потребитель, производство, стройка и цены на дома (FRED).
const props = defineProps({
  activity: { type: Object, default: () => ({}) }, // us_retail_yoy, us_indpro_yoy, us_housing, us_homeprice_yoy
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'activity')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянется активность США из FRED'
const monthOf = (p) => (p ? `${p.d.slice(5, 7)}.${p.d.slice(0, 4)}` : '')
const retail = computed(() => lastPoint(props.activity.us_retail_yoy))
const indpro = computed(() => lastPoint(props.activity.us_indpro_yoy))
const housing = computed(() => lastPoint(props.activity.us_housing))
const homes = computed(() => lastPoint(props.activity.us_homeprice_yoy))
const has = computed(() => hasPoints(props.activity))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Активность США" :running="running" @refresh="refresh" />

    <MacroPanel title="Розничные продажи и промышленное производство (к прошлому году)" :series="activity" :today="today" :ids="['us_retail_yoy', 'us_indpro_yoy']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Розница — спрос потребителя, промышленность — предложение">
      <template #summary>
        <span v-if="retail" class="text-slate-400">розница {{ monthOf(retail) }} <b class="tabular-nums text-sky-300">{{ plainPct(retail.v) }}</b></span>
        <span v-if="indpro" class="text-slate-400">промышленность <b class="tabular-nums text-amber-300">{{ plainPct(indpro.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Начатое строительство жилья (в год)" unit="тыс." :series="activity" :today="today" :ids="['us_housing']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Стройка первой реагирует на высокие ставки">
      <template #summary>
        <span v-if="housing" class="text-slate-400">{{ monthOf(housing) }} <b class="tabular-nums text-violet-300">{{ unitText(housing.v, 'тыс.') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Цены на дома (Кейс — Шиллер, к прошлому году)" :series="activity" :today="today" :ids="['us_homeprice_yoy']" :empty="EMPTY">
      <template #summary>
        <span v-if="homes" class="text-slate-400">{{ monthOf(homes) }} <b class="tabular-nums text-emerald-400">{{ plainPct(homes.v) }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса). Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

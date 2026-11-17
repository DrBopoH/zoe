<script setup>
import { computed } from 'vue'
import CurveSnapshot from '../components/CurveSnapshot.vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, curveState, hasPoints } from '../utils/macro.js'

// Экран «Кривая Treasury»: доходности по срокам (FRED) и разница длинных и коротких ставок. Это картина, а не сигнал к действию.
const props = defineProps({
  curve: { type: Object, default: () => ({}) }, // ust_1m … ust_30y, us_spread3m и ust_10y
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'yields')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянется кривая доходности Treasury из FRED'
const spread = computed(() => lastPoint(props.curve.us_spread3m))
const state = computed(() => curveState(spread.value?.v))
const STATE_TEXT = { inverted: 'инвертирована: короткие ставки выше длинных, исторически это опережало рецессии', flat: 'почти плоская', normal: 'нормальная: длинные ставки выше коротких' }
const has = computed(() => hasPoints(props.curve))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Кривая Treasury" :running="running" @refresh="refresh" />

    <CurveSnapshot :series="curve" :today="today" />

    <MacroPanel title="Доходность Treasury по срокам" :series="curve" :today="today" :ids="['ust_3m', 'ust_2y', 'ust_10y', 'ust_30y']" :empty="EMPTY" />

    <MacroPanel title="Кривая: 10-летние минус 3-месячные Treasury" unit="п. п." :series="curve" :today="today" :ids="['us_spread3m']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Ниже нуля кривая «инвертирована»">
      <template #summary>
        <span v-if="spread" class="text-slate-400">Сейчас <b class="tabular-nums" :class="spread.v < 0 ? 'text-rose-400' : 'text-emerald-400'">{{ plainPct(spread.v) }}</b></span>
        <span v-if="state" class="text-slate-500">кривая {{ STATE_TEXT[state] }}</span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса), дневные, на графиках — последнее значение недели. Подсказки — как кривую читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

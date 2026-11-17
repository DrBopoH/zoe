<script setup>
import { numText } from '../utils/numfmt.js'
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, creditLevel, hasPoints } from '../utils/macro.js'

// Экран «Кредит и доллар»: что требуют кредиторы за риск (спреды), насколько жёсткие финансовые условия, широкий доллар и госдолг (FRED).
const props = defineProps({
  credit: { type: Object, default: () => ({}) }, // us_hy, us_ig, us_nfci, us_usd, us_debt_gdp
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'credit')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся кредит и доллар США из FRED'
const num = (v) => numText(v, 2)
const hy = computed(() => lastPoint(props.credit.us_hy))
const ig = computed(() => lastPoint(props.credit.us_ig))
const level = computed(() => creditLevel(hy.value?.v))
const LEVEL = { calm: 'спокойно', wary: 'настороженно', stress: 'стресс' }
const nfci = computed(() => lastPoint(props.credit.us_nfci))
const usd = computed(() => lastPoint(props.credit.us_usd))
const debt = computed(() => lastPoint(props.credit.us_debt_gdp))
const has = computed(() => hasPoints(props.credit))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Кредит и доллар США" :running="running" @refresh="refresh" />

    <MacroPanel title="Кредитные спреды: высокодоходные и инвестиционные облигации" unit="п. п." :series="credit" :today="today" :ids="['us_hy', 'us_ig']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Спред — доплата сверх Treasury; растёт, когда кредиторы боятся. Источник отдаёт только последние 3 года">
      <template #summary>
        <span v-if="hy" class="text-slate-400">HY <b class="tabular-nums text-rose-400">{{ num(hy.v) }} п. п.</b><span v-if="level" class="text-slate-600"> · {{ LEVEL[level] }}</span></span>
        <span v-if="ig" class="text-slate-400">IG <b class="tabular-nums text-sky-300">{{ num(ig.v) }} п. п.</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Финансовые условия (NFCI, Чикаго)" unit="п." :series="credit" :today="today" :ids="['us_nfci']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Ноль — средние условия, выше нуля жёстче среднего, ниже мягче">
      <template #summary>
        <span v-if="nfci" class="text-slate-400">Сейчас <b class="tabular-nums" :class="nfci.v > 0 ? 'text-rose-400' : 'text-emerald-400'">{{ num(nfci.v) }}</b><span class="text-slate-600"> · {{ nfci.v > 0 ? 'жёстче среднего' : 'мягче среднего' }}</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Широкий индекс доллара" unit="п." :series="credit" :today="today" :ids="['us_usd']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Январь 2006 = 100; выше — доллар дороже к корзине валют торговых партнёров">
      <template #summary>
        <span v-if="usd" class="text-slate-400">Сейчас <b class="tabular-nums text-amber-300">{{ num(usd.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Федеральный долг к ВВП" :series="credit" :today="today" :ids="['us_debt_gdp']" :empty="EMPTY">
      <template #summary>
        <span v-if="debt" class="text-slate-400">на {{ debt.d.slice(0, 4) }} <b class="tabular-nums text-orange-300">{{ plainPct(debt.v) }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса). Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

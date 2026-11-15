<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, billionsText, hasPoints } from '../utils/macro.js'

// Экран «Ликвидность США»: сколько денег в системе (FRED). Чистая ликвидность = баланс ФРС − счёт Казначейства − обратное РЕПО.
const props = defineProps({
  liquidity: { type: Object, default: () => ({}) }, // us_netliq, us_fedbs, us_tga, us_rrp, us_m2, us_m2_yoy, us_sofr
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'liquidity')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянется ликвидность США из FRED'
const dateOf = (p) => (p ? p.d.split('-').reverse().join('.') : '')
const monthOf = (p) => (p ? `${p.d.slice(5, 7)}.${p.d.slice(0, 4)}` : '')
const net = computed(() => lastPoint(props.liquidity.us_netliq))
const fedbs = computed(() => lastPoint(props.liquidity.us_fedbs))
const tga = computed(() => lastPoint(props.liquidity.us_tga))
const rrp = computed(() => lastPoint(props.liquidity.us_rrp))
const m2 = computed(() => lastPoint(props.liquidity.us_m2))
const m2yoy = computed(() => lastPoint(props.liquidity.us_m2_yoy))
const sofr = computed(() => lastPoint(props.liquidity.us_sofr))
const has = computed(() => hasPoints(props.liquidity))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Ликвидность США" :running="running" @refresh="refresh">
      <span v-if="net" class="text-slate-500">баланс ФРС: среда {{ dateOf(net) }}</span>
    </MarketHeader>

    <MacroPanel title="Чистая ликвидность и баланс ФРС" unit="млрд $" :series="liquidity" :today="today" :ids="['us_netliq', 'us_fedbs']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Чистая ликвидность — деньги ФРС в системе за вычетом счёта Казначейства и обратного РЕПО; её рост рынки исторически любят">
      <template #summary>
        <span v-if="net" class="text-slate-400">чистая <b class="tabular-nums text-emerald-400">{{ billionsText(net.v) }}</b></span>
        <span v-if="fedbs" class="text-slate-400">баланс ФРС <b class="tabular-nums text-violet-300">{{ billionsText(fedbs.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Счёт Казначейства и обратное РЕПО" unit="млрд $" :series="liquidity" :today="today" :ids="['us_tga', 'us_rrp']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Рост этих счетов забирает деньги из системы, падение возвращает">
      <template #summary>
        <span v-if="tga" class="text-slate-400">Казначейство <b class="tabular-nums text-pink-300">{{ billionsText(tga.v) }}</b></span>
        <span v-if="rrp" class="text-slate-400">РЕПО <b class="tabular-nums text-amber-300">{{ billionsText(rrp.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Денежная масса M2 (к прошлому году)" :series="liquidity" :today="today" :ids="['us_m2_yoy']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Ниже нуля M2 сжимается: это бывало редко">
      <template #summary>
        <span v-if="m2yoy" class="text-slate-400">{{ monthOf(m2yoy) }} <b class="tabular-nums" :class="m2yoy.v >= 0 ? 'text-sky-300' : 'text-rose-400'">{{ plainPct(m2yoy.v) }}</b></span>
        <span v-if="m2" class="text-slate-400">всего <b class="tabular-nums text-sky-300">{{ billionsText(m2.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Ставка SOFR" :series="liquidity" :today="today" :ids="['us_sofr']" :empty="EMPTY">
      <template #summary>
        <span v-if="sofr" class="text-slate-400">Сейчас <b class="tabular-nums text-sky-300">{{ plainPct(sofr.v) }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса), баланс и счёт Казначейства раз в неделю, по средам. Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

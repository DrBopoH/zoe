<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, signedPct, drawdown, athInfo, changeSince, unitText } from '../utils/macro.js'

// Экран «Биржа США»: S&P 500, NASDAQ Composite и Dow Jones (FRED, последнее закрытие недели) и просадка от максимума.
// FRED даёт S&P 500 и Dow примерно за 10 лет, NASDAQ — с 1971 года: «максимум» считается по доступной истории.
// Не прогноз и не совет: картина того, где рынок относительно своих пиков.
const props = defineProps({
  stocks: { type: Object, default: () => ({}) }, // stock_sp500, stock_nasdaq, stock_dow
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh, startError } = useMarketRefresh(() => 'stocks')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся индексы S&P 500, NASDAQ и Dow Jones'

const withDd = computed(() => ({
  ...props.stocks,
  stock_sp500_dd: drawdown(props.stocks.stock_sp500),
  stock_nasdaq_dd: drawdown(props.stocks.stock_nasdaq),
  stock_dow_dd: drawdown(props.stocks.stock_dow)
}))
const sp = computed(() => lastPoint(props.stocks.stock_sp500))
const ath = computed(() => athInfo(props.stocks.stock_sp500))
const y1 = computed(() => changeSince(props.stocks.stock_sp500, props.today, 1))
const nas = computed(() => athInfo(props.stocks.stock_nasdaq))
const dow = computed(() => athInfo(props.stocks.stock_dow))
const dateOf = (iso) => (iso ? iso.split('-').reverse().join('.') : '')
const pt = (id) => lastPoint(props.stocks[id])
const has = computed(() => Object.values(props.stocks).some((a) => a && a.length))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Биржа США</h2>
      <span v-if="sp" class="text-slate-500">закрытие недели · последняя точка {{ dateOf(sp.d) }}</span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>
    <div v-if="startError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError }}</div>

    <MacroPanel title="S&P 500, пункты" unit="п." :series="stocks" :today="today" :ids="['stock_sp500']" benner :empty="EMPTY">
      <template #summary>
        <span v-if="sp" class="text-slate-400">Сейчас <b class="text-sky-300 tabular-nums">{{ unitText(sp.v, 'п.') }}</b></span>
        <span v-if="ath" class="text-slate-400" :title="`Максимум по закрытию недели: ${unitText(ath.ath, 'п.')}, ${dateOf(ath.date)}`">От максимума <b class="tabular-nums" :class="ath.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(ath.fromAth) }}</b></span>
        <span v-if="y1 !== null" class="text-slate-400">За год <b class="tabular-nums" :class="y1 >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(y1) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="NASDAQ Composite, пункты" unit="п." :series="stocks" :today="today" :ids="['stock_nasdaq']" benner :empty="EMPTY">
      <template #summary>
        <span v-if="pt('stock_nasdaq')" class="text-slate-400">Сейчас <b class="text-violet-300 tabular-nums">{{ unitText(pt('stock_nasdaq').v, 'п.') }}</b></span>
        <span v-if="nas" class="text-slate-400">От максимума <b class="tabular-nums" :class="nas.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(nas.fromAth) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Dow Jones, пункты" unit="п." :series="stocks" :today="today" :ids="['stock_dow']" benner :empty="EMPTY">
      <template #summary>
        <span v-if="pt('stock_dow')" class="text-slate-400">Сейчас <b class="text-amber-300 tabular-nums">{{ unitText(pt('stock_dow').v, 'п.') }}</b></span>
        <span v-if="dow" class="text-slate-400">От максимума <b class="tabular-nums" :class="dow.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(dow.fromAth) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Просадка от максимума (по доступной истории)" :series="withDd" :today="today" :ids="['stock_sp500_dd', 'stock_nasdaq_dd', 'stock_dow_dd']" benner :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. 0 % — индекс на максимуме, −20 % — медвежий рынок. У S&P 500 и Dow история FRED около 10 лет">
      <template #summary>
        <span v-if="ath" class="text-slate-400">S&amp;P 500 <b class="tabular-nums" :class="ath.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ plainPct(ath.fromAth) }}</b> от пика</span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Данные: FRED (S&amp;P 500, NASDAQ Composite, Dow Jones), закрытие недели. Это картина рынка, а не прогноз и не совет.</div>
  </div>
</template>

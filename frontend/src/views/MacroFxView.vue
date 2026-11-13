<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { goldUsd, annualChange, realRate, valueAt, lastPoint, signedPct, plainPct, unitText } from '../utils/macro.js'

// Экран «Курсы»: доллар, евро и золото к гривне (официальные курсы НБУ) и золото в долларах.
// Из них в сводке: насколько гривна дешевела в среднем за год и что это делает с доходностью гривневых бумаг в долларах.
const props = defineProps({
  fx: { type: Object, default: () => ({}) }, // ряды usd_uah, eur_uah, xau_uah
  macro: { type: Object, default: () => ({}) }, // ставка НБУ — для оценки «в долларах»
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'fx')
const all = computed(() => ({ ...props.fx, xau_usd: goldUsd(props.fx) }))
const USD_EUR = ['usd_uah', 'eur_uah']
const GOLD_UAH = ['xau_uah']
const GOLD_USD = ['xau_usd']

const usd = computed(() => lastPoint(props.fx.usd_uah))
const eur = computed(() => lastPoint(props.fx.eur_uah))
const gold = computed(() => lastPoint(props.fx.xau_uah))
const goldD = computed(() => lastPoint(all.value.xau_usd))
const dep = (years) => annualChange(props.fx.usd_uah, props.today, years)
const policy = computed(() => valueAt(props.macro.policy_rate, props.today))
// Гривневая ставка в долларах, если гривна и дальше будет дешеветь так же, как в среднем за период
const inUsd = (years) => (policy.value !== null && dep(years) !== null ? realRate(policy.value, dep(years)) : null)
const goldGrowth = (years) => annualChange(all.value.xau_usd, props.today, years)
const dateText = (iso) => (iso ? iso.split('-').reverse().join('.') : '')
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Курсы" :running="running" @refresh="refresh">
      <span v-if="usd" class="text-slate-500">курс НБУ на {{ dateText(usd.d) }}</span>
    </MarketHeader>

    <MacroPanel title="Доллар и евро к гривне" :series="all" :today="today" :ids="USD_EUR" unit="₴" empty="Нет данных. Нажми «Обновить»: подтянутся курсы НБУ">
      <template #summary>
        <span class="text-slate-400">Доллар <b class="text-emerald-400 tabular-nums">{{ unitText(usd?.v ?? null, '₴') }}</b></span>
        <span class="text-slate-400">Евро <b class="text-sky-300 tabular-nums">{{ unitText(eur?.v ?? null, '₴') }}</b></span>
        <span v-if="dep(1) !== null" class="text-slate-400" title="Среднегодовое подорожание доллара в гривне (сложный процент)">Доллар дорожал: <b class="text-amber-300 tabular-nums">{{ signedPct(dep(1)) }}</b> за год<template v-if="dep(5) !== null">, <b class="text-amber-300 tabular-nums">{{ signedPct(dep(5)) }}</b> в год за 5 л.</template><template v-if="dep(10) !== null">, <b class="text-amber-300 tabular-nums">{{ signedPct(dep(10)) }}</b> в год за 10 л.</template></span>
      </template>
    </MacroPanel>

    <section v-if="inUsd(5) !== null" class="text-xs bg-slate-900/60 border border-slate-800 rounded px-4 py-3 space-y-1">
      <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Гривневая ставка в долларах</div>
      <div class="text-slate-400">При облікової {{ plainPct(policy) }} и девальвации как в среднем за 5 лет ({{ signedPct(dep(5)) }} в год) гривневая бумага даёт в долларах <b class="tabular-nums" :class="inUsd(5) >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(inUsd(5)) }}</b> в год<template v-if="inUsd(10) !== null">; при девальвации как за 10 лет ({{ signedPct(dep(10)) }}) — <b class="tabular-nums" :class="inUsd(10) >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(inUsd(10)) }}</b></template>.</div>
      <div class="text-slate-600">Это не прогноз, а «если курс пойдёт так же, как шёл». Сравнивай с доходностью долларовых бумаг (US) и ОВДП в USD.</div>
    </section>

    <MacroPanel title="Золото, гривен за унцию" :series="all" :today="today" :ids="GOLD_UAH" unit="₴" empty="Нет данных. Нажми «Обновить»: подтянутся курсы НБУ">
      <template #summary>
        <span class="text-slate-400">Унция <b class="text-amber-300 tabular-nums">{{ unitText(gold?.v ?? null, '₴') }}</b></span>
        <span v-if="gold" class="text-slate-400" title="31,1035 г в тройской унции">Грамм <b class="text-amber-300 tabular-nums">{{ unitText(gold.v / 31.1035, '₴') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Золото, долларов за унцию" :series="all" :today="today" :ids="GOLD_USD" unit="$" hint="Цена в гривнах, поделённая на курс доллара: показывает, как золото росло само по себе, без влияния девальвации гривны">
      <template #summary>
        <span class="text-slate-400">Унция <b class="text-amber-300 tabular-nums">{{ unitText(goldD?.v ?? null, '$') }}</b></span>
        <span v-if="goldGrowth(5) !== null" class="text-slate-400">Рост золота в долларах: <b class="tabular-nums" :class="goldGrowth(5) >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(goldGrowth(5)) }}</b> в год за 5 л.<template v-if="goldGrowth(10) !== null">, <b class="tabular-nums" :class="goldGrowth(10) >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(goldGrowth(10)) }}</b> за 10 л.</template></span>
      </template>
    </MacroPanel>
  </div>
</template>

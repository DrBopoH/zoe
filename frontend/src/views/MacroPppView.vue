<script setup>
import { fixedText, intText } from '../utils/numfmt.js'
import { computed, ref } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { signedPct, plainPct, hasPoints, dateOf } from '../utils/macro.js'
import { PPP_COUNTRIES, countryOf, pppModel, trailingInflation, levelRows, burgerRows, compareCountries, realReturn } from '../utils/pppview.js'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'

// Экран «Покупательная способность»: что происходит с деньгами страны за N лет (инфляция), как менялся реальный курс к доллару,
// во сколько раз цены в стране дешевле или дороже американских (PPP), Big Mac, сравнение двух стран и реальная доходность.
// Данные: Всемирный банк (годовые, последний год 2025), FRED (цены США помесячно), The Economist (Big Mac). Не прогноз и не совет.
const props = defineProps({
  ppp: { type: Object, default: () => ({}) }, // ряды источника «ppp»
  macro: { type: Object, default: () => ({}) }, // ряды НБУ: годовая инфляция Украины продолжает индекс после последнего года Всемирного банка
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'ppp')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся индексы цен, PPP и Big Mac'

const cc = ref('ua')
const depositYears = ref(10)
const amount = ref(1000)
const ccB = ref('pl')
const salary = ref(30000)
const nominal = ref(15)

const country = computed(() => countryOf(cc.value))
const sym = computed(() => country.value?.sym || '')
const has = computed(() => hasPoints(props.ppp))
const model = computed(() => pppModel({ series: props.ppp, macro: props.macro, cc: cc.value, today: props.today, amount: Number(amount.value) || 0 }))
const modelB = computed(() => (ccB.value !== cc.value ? pppModel({ series: props.ppp, macro: props.macro, cc: ccB.value, today: props.today, amount: 1 }) : null))
const charts = computed(() => {
  const s = { [`pw_${cc.value}`]: model.value.power }
  if (modelB.value) s[`pw_${ccB.value}`] = modelB.value.power
  s[`rerusd_${cc.value}`] = model.value.rer
  return s
})
const powerIds = computed(() => [`pw_${cc.value}`, ...(modelB.value ? [`pw_${ccB.value}`] : [])])
const levels = computed(() => levelRows(props.ppp, props.today))
const burgers = computed(() => burgerRows(props.ppp, props.today))
const cmp = computed(() => compareCountries({ series: props.ppp, a: cc.value, b: ccB.value, salaryA: Number(salary.value) || 0, today: props.today }))
const symB = computed(() => countryOf(ccB.value)?.sym || '')
const trail = computed(() => trailingInflation({ series: props.ppp, macro: props.macro, cc: cc.value, years: depositYears.value, today: props.today }))
const rr = computed(() => realReturn({ nominalPct: Number(nominal.value), inflationPct: trail.value.pp?.perYear ?? NaN, years: trail.value.years, amount: Number(amount.value) || 0 }))
const fromYear = computed(() => model.value.from?.slice(0, 4) || '')
const span = computed(() => Math.round(model.value.years))
const lastYear = computed(() => {
  const d = props.ppp[`ppp_cpi_${cc.value === 'ez' ? 'de' : cc.value}`]?.slice(-1)[0]?.d
  return d ? d.slice(0, 4) : null
})

const money = (v, s = sym.value) => (v === null || v === undefined || !isFinite(v) ? '—' : `${intText(v)} ${s}`)
const num = (v, d = 2) => (v === null || v === undefined || !isFinite(v) ? '—' : fixedText(v, d))
const tone = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Покупательная способность" :running="running" @refresh="refresh">
      <span v-if="lastYear" class="text-slate-500">цены до {{ lastYear }} года, дальше — по текущей инфляции</span>
    </MarketHeader>

    <UiCard class="p-3 text-xs">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <label class="flex items-center gap-2 text-slate-400">Страна
          <select v-model="cc" class="ui-input !bg-slate-900 !w-44"><option v-for="c in PPP_COUNTRIES" :key="c.cc" :value="c.cc">{{ c.name }}</option></select>
        </label>
        <label class="flex items-center gap-2 text-slate-400">Сумма
          <input v-model.number="amount" type="number" min="1" step="100" class="ui-input !bg-slate-900 !w-28 text-right" />
          <b class="text-slate-300">{{ sym }}</b>
        </label>
      </div>
      <div v-if="!has" class="mt-3 text-slate-500">{{ EMPTY }}</div>
      <div v-else-if="model.pp" class="mt-3 space-y-1 text-slate-400">
        <div>Цены с {{ fromYear }} года ({{ span }} лет) выросли на <b class="tabular-nums" :class="tone(-model.pp.inflation)">{{ plainPct(model.pp.inflation) }}</b>, в среднем <b class="tabular-nums text-slate-200">{{ plainPct(model.pp.perYear) }}</b> в год.</div>
        <div><b class="text-slate-100 tabular-nums">{{ money(Number(amount) || 0) }}</b> в {{ fromYear }} году — это <b class="text-amber-300 tabular-nums">{{ money(model.needNow) }}</b> в сегодняшних ценах. Сегодняшние {{ money(Number(amount) || 0) }} купят столько, сколько тогда <b class="text-rose-400 tabular-nums">{{ money(model.worthThen) }}</b>.</div>
      </div>
      <div v-else class="mt-3 text-slate-500">По этой стране нет индекса цен.</div>
    </UiCard>

    <MacroPanel :title="`Сколько осталось от покупательной способности денег (старт = 100 %)`" unit="%" :series="charts" :today="today" :ids="powerIds" :empty="EMPTY" hint="Деньги дешевеют на размер накопленной инфляции. Вторая линия — страна для сравнения ниже" />
    <MacroPanel v-if="cc !== 'us'" :title="`Реальный курс к доллару: ${country?.name} (старт = 100)`" unit="п." :series="charts" :today="today" :ids="[`rerusd_${cc}`]" :empty="EMPTY" hint="Курс с поправкой на инфляцию двух стран. Выше 100 — валюта реально дешевле, чем в начале ряда; ниже — дороже" />

    <UiCard class="overflow-x-auto">
      <div class="px-3 py-2 text-[11px] text-slate-500">Уровень цен относительно США: PPP-коэффициент к рыночному курсу. 0,30 — цены на 70 % ниже американских, 1,20 — на 20 % выше. Доходы на душу — в международных долларах (что можно купить) и в обычных.</div>
      <table v-if="levels.length" class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr class="border-t border-slate-800/70"><th class="px-3 py-1.5 text-left font-bold">Страна</th><th class="px-2 text-right font-bold">Цены к США</th><th class="px-2 text-right font-bold">Отклонение</th><th class="px-2 text-right font-bold">Доход PPP, $</th><th class="px-3 text-right font-bold">Доход по курсу, $</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="r in levels" :key="r.cc" :class="r.cc === cc && 'bg-slate-800/40'">
            <td class="px-3 py-1.5 text-slate-300">{{ r.name }}</td>
            <td class="px-2 text-right text-slate-100">{{ num(r.level) }}</td>
            <td class="px-2 text-right" :class="tone(r.dev)">{{ r.dev === null ? '—' : signedPct(r.dev) }}</td>
            <td class="px-2 text-right text-slate-300">{{ r.gdpPpp === null ? '—' : intText(r.gdpPpp) }}</td>
            <td class="px-3 text-right text-slate-300">{{ r.gdpNom === null ? '—' : intText(r.gdpNom) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>{{ EMPTY }}</UiEmpty>
    </UiCard>

    <UiCard class="overflow-x-auto">
      <div class="px-3 py-2 text-[11px] text-slate-500">Big Mac Index (The Economist): цена бургера в долларах по рыночному курсу и отклонение от американской цены. «Бургеров в месяц» — среднедушевой доход ÷ 12 ÷ цена бургера.</div>
      <table v-if="burgers.length" class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr class="border-t border-slate-800/70"><th class="px-3 py-1.5 text-left font-bold">Страна</th><th class="px-2 text-right font-bold">Цена, $</th><th class="px-2 text-right font-bold">К США</th><th class="px-3 text-right font-bold">Бургеров в месяц</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="r in burgers" :key="r.cc" :class="r.cc === cc && 'bg-slate-800/40'">
            <td class="px-3 py-1.5 text-slate-300">{{ r.name }} <span class="text-slate-600">{{ r.date ? dateOf(r.date) : '' }}</span></td>
            <td class="px-2 text-right text-slate-100">{{ num(r.price) }}</td>
            <td class="px-2 text-right" :class="tone(r.raw)">{{ r.raw === null ? '—' : signedPct(r.raw) }}</td>
            <td class="px-3 text-right text-slate-300">{{ r.perMonth === null ? '—' : intText(r.perMonth) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>{{ EMPTY }}</UiEmpty>
    </UiCard>

    <UiCard class="p-3 text-xs space-y-2">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span class="text-slate-400">Две страны: оклад в стране «{{ country?.name }}»</span>
        <input v-model.number="salary" type="number" min="1" step="1000" class="ui-input !bg-slate-900 !w-28 text-right" />
        <b class="text-slate-300">{{ sym }}</b>
        <label class="flex items-center gap-2 text-slate-400">и страна для сравнения
          <select v-model="ccB" class="ui-input !bg-slate-900 !w-44"><option v-for="c in PPP_COUNTRIES" :key="c.cc" :value="c.cc">{{ c.name }}</option></select>
        </label>
      </div>
      <div v-if="cmp.inB !== null && ccB !== cc" class="space-y-1 text-slate-400">
        <div>По местным ценам такой оклад равен <b class="text-emerald-400 tabular-nums">{{ money(cmp.inB, symB) }}</b> в стране «{{ countryOf(ccB)?.name }}» (на столько же всего хватит).</div>
        <div v-if="cmp.inBMarket !== null">По рыночному курсу это только <b class="text-slate-200 tabular-nums">{{ money(cmp.inBMarket, symB) }}</b> ({{ money(cmp.usdA, '$') }}). Разница — то, насколько цены в двух странах не соответствуют курсу.</div>
        <div v-if="cmp.income.ppp !== null">Доход на душу в «{{ country?.name }}» составляет <b class="text-slate-200 tabular-nums">{{ num(cmp.income.ppp * 100, 0) }} %</b> от дохода в «{{ countryOf(ccB)?.name }}» по покупательной способности и <b class="text-slate-200 tabular-nums">{{ cmp.income.nominal === null ? '—' : num(cmp.income.nominal * 100, 0) }} %</b> по курсу.</div>
      </div>
      <div v-else class="text-slate-500">Выбери другую страну для сравнения.</div>
    </UiCard>

    <UiCard class="p-3 text-xs space-y-2">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span class="text-slate-400">Реальная доходность: вклад на</span>
        <select v-model.number="depositYears" class="ui-input !bg-slate-900 !w-24"><option v-for="y in [5, 10, 15, 20, 25]" :key="y" :value="y">{{ y }} лет</option></select>
        <span class="text-slate-400">под</span>
        <input v-model.number="nominal" type="number" step="0.5" class="ui-input !bg-slate-900 !w-20 text-right" /><b class="text-slate-300">% годовых</b>
        <span class="text-slate-500">при инфляции страны «{{ country?.name }}» за эти годы: {{ trail.pp ? plainPct(trail.pp.perYear) : '—' }} в год</span>
      </div>
      <div v-if="rr" class="space-y-1 text-slate-400">
        <div>Реальная доходность: <b class="tabular-nums" :class="tone(rr.real)">{{ signedPct(rr.real) }}</b> в год — {{ rr.beats ? 'деньги растут быстрее цен' : 'вклад не обгоняет инфляцию, покупательная способность падает' }}.</div>
        <div>{{ money(Number(amount) || 0) }} вырастут до <b class="text-slate-200 tabular-nums">{{ money(rr.grown) }}</b>, но по покупательной способности это <b class="tabular-nums" :class="tone(rr.today - (Number(amount) || 0))">{{ money(rr.today) }}</b> в сегодняшних деньгах.</div>
        <div class="text-slate-600">Безубыточный номинальный рост равен инфляции: ниже — теряешь, выше — зарабатываешь. Налоги и комиссии здесь не учтены.</div>
      </div>
      <div v-else class="text-slate-500">Нужна инфляция за выбранные годы.</div>
    </UiCard>

    <div v-if="has" class="text-[11px] text-slate-600">Данные: Всемирный банк (индексы цен, курс, PPP-коэффициент, доход на душу, годовые), FRED (цены США помесячно), The Economist (Big Mac, дважды в год). У еврозоны цены и PPP — немецкие. Это справочные расчёты, а не прогноз и не совет.</div>
  </div>
</template>

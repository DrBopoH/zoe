<script setup>
import { dmy, numText } from '../utils/numfmt.js'
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { goldUsd, annualChange, realRate, valueAt, lastPoint, signedPct, plainPct, unitText } from '../utils/macro.js'
import { MATRIX_CCY, crossMatrix, dxyIndex, fxWorldRows, rateText } from '../utils/fxworld.js'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'

// Экран «Курсы»: доллар, евро и золото к гривне (официальные курсы НБУ) и золото в долларах.
// Из них в сводке: насколько гривна дешевела в среднем за год и что это делает с доходностью гривневых бумаг в долларах.
const props = defineProps({
  fx: { type: Object, default: () => ({}) }, // ряды usd_uah, eur_uah, xau_uah
  macro: { type: Object, default: () => ({}) }, // ставка НБУ — для оценки «в долларах»
  countries: { type: Array, default: () => [] }, // реестр стран с сервера: курс, ставка и реальный курс каждой
  cseries: { type: Object, default: () => ({}) }, // ряды карточек стран (ставки)
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'fx')
const all = computed(() => ({ ...props.fx, xau_usd: goldUsd(props.fx), fx_dxy: dxyIndex(props.fx) }))
// Мировые валюты: таблица по реестру стран, кросс-курсы через гривну, индекс доллара и реальные курсы (BIS)
const merged = computed(() => ({ ...props.cseries, ...props.fx }))
const rows = computed(() => fxWorldRows(props.countries, merged.value, props.today))
const matrix = computed(() => crossMatrix(props.fx, props.today))
const REER_MAJOR = ['reer_us', 'reer_ez', 'reer_gb', 'reer_jp', 'reer_cn', 'reer_ch']
const REER_OTHER = ['reer_tr', 'reer_ru', 'reer_pl', 'reer_ca', 'reer_no', 'reer_se', 'reer_kr', 'reer_tw']
const DXY_ID = ['fx_dxy']
const dxyLast = computed(() => lastPoint(all.value.fx_dxy))
const chgClass = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
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
const dateText = (iso) => (iso ? dmy(iso) : '')
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

    <UiCard class="overflow-x-auto">
      <div class="px-3 py-2 text-[11px] text-slate-500">Мировые валюты · курсы НБУ, ставки центробанков и реальные эффективные курсы (BIS). «За год»: плюс — валюта окрепла к доллару. «Разница»: ставка страны минус ставка ФРС (чем больше, тем выгоднее держать валюту, если курс не упадёт).</div>
      <table v-if="rows.length" class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr class="border-t border-slate-800/70"><th class="px-3 py-1.5 text-left font-bold">Валюта</th><th class="px-2 text-right font-bold">₴</th><th class="px-2 text-right font-bold">$</th><th class="px-2 text-right font-bold">За год</th><th class="px-2 text-right font-bold">Ставка</th><th class="px-2 text-right font-bold">Разница</th><th class="px-2 text-right font-bold">REER</th><th class="px-3 text-right font-bold">REER за год</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="r in rows" :key="r.cc">
            <td class="px-3 py-1.5 text-slate-300"><b>{{ r.currency }}</b> <span class="text-slate-600">{{ r.name }}</span></td>
            <td class="px-2 text-right text-slate-100">{{ rateText(r.uah) }}</td>
            <td class="px-2 text-right text-slate-100">{{ rateText(r.usdPer) }}</td>
            <td class="px-2 text-right" :class="chgClass(r.chg1y)">{{ r.chg1y === null ? '—' : signedPct(r.chg1y) }}</td>
            <td class="px-2 text-right text-slate-300">{{ r.policy === null ? '—' : plainPct(r.policy) }}</td>
            <td class="px-2 text-right" :class="chgClass(r.carry)">{{ r.carry === null ? '—' : signedPct(r.carry) }}</td>
            <td class="px-2 text-right text-slate-300">{{ r.reer === null ? '—' : numText(r.reer, 1) }}</td>
            <td class="px-3 text-right" :class="chgClass(r.reerChg1y)">{{ r.reerChg1y === null ? '—' : signedPct(r.reerChg1y) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>Нет данных. Нажми «Обновить»: подтянутся курсы и реальные курсы.</UiEmpty>
    </UiCard>

    <MacroPanel title="Индекс доллара (расчёт по курсам НБУ)" :series="all" :today="today" :ids="DXY_ID" unit="п." hint="Формула ICE (DXY): евро, иена, фунт, канадский доллар, шведская крона и франк через курсы к гривне. Растёт — доллар дорожает к корзине" empty="Нет данных. Нажми «Обновить»: подтянутся курсы НБУ">
      <template #summary>
        <span v-if="dxyLast" class="text-slate-400">Сейчас <b class="text-emerald-400 tabular-nums">{{ unitText(dxyLast.v, 'п.') }}</b></span>
      </template>
    </MacroPanel>

    <UiCard class="overflow-x-auto">
      <div class="px-3 py-2 text-[11px] text-slate-500">Кросс-курсы: сколько единиц валюты в столбце стоит одна единица валюты в строке (через гривну, на сегодня).</div>
      <table class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr class="border-t border-slate-800/70"><th class="px-3 py-1.5"></th><th v-for="c in MATRIX_CCY" :key="c.code" class="px-2 text-right font-bold">{{ c.code }}</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="row in matrix" :key="row.code">
            <th class="px-3 py-1.5 text-left font-bold text-slate-400">{{ row.code }}</th>
            <td v-for="(v, i) in row.cells" :key="i" class="px-2 text-right" :class="v === null ? 'text-slate-700' : 'text-slate-200'">{{ v === null ? '·' : rateText(v) }}</td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <MacroPanel title="Реальные эффективные курсы: крупные валюты (BIS, индекс)" :series="fx" :today="today" :ids="REER_MAJOR" unit="п." hint="Курс к корзине торговых партнёров с поправкой на инфляцию. Выше — валюта реально дороже" empty="Нет данных. Нажми «Обновить»: подтянутся реальные курсы" />
    <MacroPanel title="Реальные эффективные курсы: остальные (BIS, индекс)" :series="fx" :today="today" :ids="REER_OTHER" unit="п." hint="Турция и Россия — на другой шкале: колесо мыши приближает" empty="Нет данных. Нажми «Обновить»: подтянутся реальные курсы" />
  </div>
</template>

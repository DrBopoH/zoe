<script setup>
import { computed, ref } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, signedPct, dateOf } from '../utils/macro.js'
import { COMMODITY_GROUPS, commodityView, commodityRows, groupIds, priceText } from '../utils/commodities.js'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'
import UiNotice from '../components/ui/UiNotice.vue'

// Экран «Сырьё»: нефть, газ, металлы, зерно, сахар и кофе (FRED, Всемирный банк) и четыре драгоценных металла из курсов НБУ.
// Переключатель «$ / ₴» пересчитывает всё по курсу доллара НБУ на дату каждой точки.
const props = defineProps({
  com: { type: Object, default: () => ({}) }, // ряды com_* (источник «commodities») и xau_uah, xag_uah, xpt_uah, xpd_uah, usd_uah (источник «fx»)
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'commodities')
const mode = ref('usd')
const sym = computed(() => (mode.value === 'uah' ? '₴' : '$'))
const view = computed(() => commodityView(props.com, mode.value))
const rows = computed(() => commodityRows(view.value, props.today))
const hasAny = computed(() => rows.value.some((r) => r.last))
const lastDate = computed(() => rows.value.map((r) => r.last?.d).filter(Boolean).sort().pop() || null)
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся цены сырья (металлы приходят вместе с курсами в разделе «Курсы»)'
const chg = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
const usdMissing = computed(() => mode.value === 'uah' && !props.com.usd_uah?.length)
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Сырьё" :running="running" @refresh="refresh">
      <span v-if="lastDate" class="text-slate-500">последняя точка {{ dateOf(lastDate) }}</span>
      <span class="inline-flex overflow-hidden rounded-md border border-slate-700" role="group" aria-label="Валюта цен">
        <button v-for="m in [['usd', '$'], ['uah', '₴']]" :key="m[0]" type="button" class="px-2.5 py-1 font-bold transition" :class="mode === m[0] ? 'bg-slate-700 text-slate-100' : 'text-slate-500 hover:text-slate-300'" :aria-pressed="mode === m[0]" @click="mode = m[0]">{{ m[1] }}</button>
      </span>
    </MarketHeader>

    <UiNotice v-if="usdMissing">Курса доллара пока нет: открой раздел «Курсы» и нажми «Обновить», тогда цены в гривнах появятся.</UiNotice>

    <UiCard class="overflow-x-auto">
      <table v-if="hasAny" class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr><th class="px-3 py-1.5 text-left font-bold">Товар</th><th class="px-2 text-right font-bold">За</th><th class="px-2 text-right font-bold">Цена, {{ sym }}</th><th class="px-3 text-right font-bold">За год</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="r in rows" :key="r.id">
            <td class="px-3 py-1.5 text-slate-300">{{ r.label }}</td>
            <td class="px-2 text-right text-slate-500">{{ r.per }}</td>
            <td class="px-2 text-right text-slate-100">{{ r.last ? priceText(r.last.v) : '—' }}</td>
            <td class="px-3 text-right" :class="chg(r.chg1y)">{{ r.chg1y === null ? '—' : signedPct(r.chg1y) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>{{ EMPTY }}</UiEmpty>
    </UiCard>

    <MacroPanel v-for="g in COMMODITY_GROUPS" :key="g.key" :title="`${g.title}, ${sym} за ${g.per}`" :unit="sym" :series="view" :today="today" :ids="groupIds(g.key)" :hint="g.hint" :empty="EMPTY">
      <template #summary>
        <span v-for="r in rows.filter((x) => groupIds(g.key).includes(x.id) && x.last)" :key="r.id" class="text-slate-400">{{ r.label }} <b class="text-slate-100 tabular-nums">{{ priceText(r.last.v) }} {{ sym }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="hasAny" class="text-[11px] text-slate-600">Данные: FRED (EIA — нефть и газ США, ежедневно; Всемирный банк — остальное, помесячно), НБУ (металлы). Цены в гривнах — по курсу доллара НБУ на дату точки. Это справочные цены, а не прогноз и не совет.</div>
  </div>
</template>

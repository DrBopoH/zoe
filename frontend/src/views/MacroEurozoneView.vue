<script setup>
import { numText } from '../utils/numfmt.js'
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, hasPoints, SERIES } from '../utils/macro.js'
import { memberIds, spreadIds } from '../utils/countries.js'

// Экран «Еврозона»: ставки ЕЦБ и €STR, кривая 2 и 10 лет, инфляция (общая и базовая), безработица и рост ВВП. Источники — ЕЦБ и Евростат, без ключей.
const props = defineProps({
  ez: { type: Object, default: () => ({}) }, // ez_* и eu_cpi_yoy
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'eurozone')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся ставки ЕЦБ и показатели еврозоны'
const dfr = computed(() => lastPoint(props.ez.ez_policy))
const mro = computed(() => lastPoint(props.ez.ez_mro))
const estr = computed(() => lastPoint(props.ez.ez_estr))
const y2 = computed(() => lastPoint(props.ez.ez_y2))
const y10 = computed(() => lastPoint(props.ez.ez_y10))
const slope = computed(() => (y2.value && y10.value ? y10.value.v - y2.value.v : null))
const cpi = computed(() => lastPoint(props.ez.eu_cpi_yoy))
const core = computed(() => lastPoint(props.ez.ez_cpi_core))
const u = computed(() => lastPoint(props.ez.ez_u))
const gdp = computed(() => lastPoint(props.ez.ez_gdp))
const has = computed(() => hasPoints(props.ez))
// Страны-члены: последнее значение каждой в шапке панели; цвет и подпись — из SERIES
const MEMBER_CPI = memberIds('cpi'), MEMBER_U = memberIds('u'), MEMBER_Y10 = memberIds('y10'), SPREADS = spreadIds()
const lasts = (ids) => ids.map((id) => ({ id, label: SERIES[id].label.split(':')[0].split(' −')[0], color: SERIES[id].color, p: lastPoint(props.ez[id]) })).filter((r) => r.p)
const members = computed(() => ({ cpi: lasts(MEMBER_CPI), u: lasts(MEMBER_U), y10: lasts(MEMBER_Y10), spread: lasts(SPREADS) }))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Еврозона" :running="running" @refresh="refresh" />

    <MacroPanel title="Ставки ЕЦБ и €STR" :series="ez" :today="today" :ids="['ez_policy', 'ez_mro', 'ez_estr']" :empty="EMPTY">
      <template #summary>
        <span v-if="dfr" class="text-slate-400">Депозитная <b class="tabular-nums text-violet-300">{{ plainPct(dfr.v) }}</b></span>
        <span v-if="mro" class="text-slate-400">Основная <b class="tabular-nums text-emerald-300">{{ plainPct(mro.v) }}</b></span>
        <span v-if="estr" class="text-slate-400">€STR <b class="tabular-nums text-sky-300">{{ plainPct(estr.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Доходность гособлигаций еврозоны (кривая ЕЦБ, рейтинг AAA)" :series="ez" :today="today" :ids="['ez_y2', 'ez_y10']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Это корзина лучших стран, а не облигации Германии">
      <template #summary>
        <span v-if="y2" class="text-slate-400">2 года <b class="tabular-nums text-amber-300">{{ plainPct(y2.v) }}</b></span>
        <span v-if="y10" class="text-slate-400">10 лет <b class="tabular-nums text-sky-300">{{ plainPct(y10.v) }}</b></span>
        <span v-if="slope !== null" class="text-slate-400">наклон <b class="tabular-nums" :class="slope < 0 ? 'text-rose-400' : 'text-slate-200'">{{ plainPct(slope) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Инфляция еврозоны (HICP)" :series="ez" :today="today" :ids="['eu_cpi_yoy', 'ez_cpi_core']" :empty="EMPTY">
      <template #summary>
        <span v-if="cpi" class="text-slate-400">Общая <b class="tabular-nums text-sky-300">{{ plainPct(cpi.v) }}</b></span>
        <span v-if="core" class="text-slate-400">Базовая <b class="tabular-nums text-orange-300">{{ plainPct(core.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Безработица и рост ВВП" :series="ez" :today="today" :ids="['ez_u', 'ez_gdp']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. ВВП — рост за квартал к предыдущему, с учётом сезонности">
      <template #summary>
        <span v-if="u" class="text-slate-400">Безработица <b class="tabular-nums text-sky-300">{{ plainPct(u.v) }}</b></span>
        <span v-if="gdp" class="text-slate-400">ВВП за квартал <b class="tabular-nums text-emerald-300">{{ plainPct(gdp.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Инфляция по странам еврозоны (HICP)" :series="ez" :today="today" :ids="MEMBER_CPI" :empty="EMPTY">
      <template #summary>
        <span v-for="r in members.cpi" :key="r.id" class="text-slate-400">{{ r.label }} <b class="tabular-nums" :style="{ color: r.color }">{{ plainPct(r.p.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Безработица по странам еврозоны" :series="ez" :today="today" :ids="MEMBER_U" :empty="EMPTY">
      <template #summary>
        <span v-for="r in members.u" :key="r.id" class="text-slate-400">{{ r.label }} <b class="tabular-nums" :style="{ color: r.color }">{{ plainPct(r.p.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Доходность 10 лет по странам еврозоны" :series="ez" :today="today" :ids="MEMBER_Y10" :empty="EMPTY">
      <template #summary>
        <span v-for="r in members.y10" :key="r.id" class="text-slate-400">{{ r.label }} <b class="tabular-nums" :style="{ color: r.color }">{{ plainPct(r.p.v) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Спред к Германии (10 лет)" unit="п. п." :series="ez" :today="today" :ids="SPREADS" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Разница доходностей со страной Bund — плата за риск страны внутри еврозоны">
      <template #summary>
        <span v-for="r in members.spread" :key="r.id" class="text-slate-400">{{ r.label }} <b class="tabular-nums" :style="{ color: r.color }">{{ numText(r.p.v, 2) }} п. п.</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды ЕЦБ и Евростата. Подсказки — как показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

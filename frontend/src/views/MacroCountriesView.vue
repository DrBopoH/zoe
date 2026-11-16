<script setup>
import { computed, ref, watch } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { storageGet, storageSet } from '../utils/storage.js'
import { countryRows, COUNTRY_PANELS } from '../utils/countries.js'
import { dateOf, hasPoints } from '../utils/macro.js'
import UiCard from '../components/ui/UiCard.vue'

// Экран «Страны»: переключатель стран и карточка — главные показатели одной строкой каждый; клик по строке рисует её график.
// Какие страны и ряды, решает сервер (реестр стран): новая страна появляется здесь без правок экрана.
const props = defineProps({
  countries: { type: Array, default: () => [] }, // [{cc, name, currency, metrics}]
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const KEY = 'economic_market_country'
const cc = ref(storageGet(KEY))
const country = computed(() => props.countries.find((c) => c.cc === cc.value) || props.countries[0] || null)
const { refresh } = useMarketRefresh(() => 'countries', () => country.value?.source)
watch(country, (c) => { if (c) storageSet(KEY, c.cc) })
const rows = computed(() => countryRows(country.value, props.series))
const pick = ref('')
const chosen = computed(() => rows.value.find((r) => r.key === pick.value) || rows.value.find((r) => r.last) || rows.value[0] || null)
const panels = computed(() => COUNTRY_PANELS[country.value?.cc] || [])
const has = computed(() => hasPoints(props.series))
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся показатели выбранной страны'
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Страны" :running="running" @refresh="refresh">
      <div class="flex flex-wrap gap-1" role="tablist" aria-label="Страна">
        <button
          v-for="c in countries" :key="c.cc" type="button" role="tab" :aria-selected="country?.cc === c.cc"
          class="px-2.5 py-1 rounded border text-[11px] font-bold uppercase tracking-wider transition-colors"
          :class="country?.cc === c.cc ? 'border-sky-400 text-sky-300 bg-sky-400/10' : 'border-slate-700 text-slate-500 hover:text-slate-300'"
          @click="cc = c.cc; pick = ''"
        >{{ c.name }}</button>
      </div>
    </MarketHeader>

    <div v-if="!country" class="text-xs text-slate-500">Список стран ещё не пришёл с сервера.</div>
    <template v-else>
      <UiCard as="div" class="divide-y divide-slate-800/70">
        <div class="px-3 py-2 text-[11px] text-slate-500">{{ country.name }} · валюта {{ country.currency }}</div>
        <button
          v-for="r in rows" :key="r.key" type="button"
          class="w-full flex items-baseline gap-3 px-3 py-2 text-left text-xs hover:bg-slate-800/40 transition-colors"
          :class="chosen?.key === r.key ? 'bg-slate-800/50' : ''"
          @click="pick = r.key"
        >
          <span class="text-slate-400 flex-1">{{ r.label }}</span>
          <b class="tabular-nums text-slate-100">{{ r.text }}</b>
          <span class="w-20 text-right text-[11px] text-slate-600 tabular-nums">{{ r.last ? dateOf(r.last.d) : '' }}</span>
        </button>
      </UiCard>

      <MacroPanel v-if="chosen" :title="`${country.name}: ${chosen.label.toLowerCase()}`" :unit="chosen.unit" :series="series" :today="today" :ids="[chosen.id]" :empty="EMPTY" />
      <MacroPanel v-for="p in panels" :key="p.title" :title="p.title" :unit="p.unit" :series="series" :today="today" :ids="p.ids" :empty="EMPTY" />
      <div v-if="has" class="text-[11px] text-slate-600">Значения — последние в кеше; даты у показателей разные (ставки — день, инфляция и труд — месяц, ВВП — квартал).</div>
    </template>
  </div>
</template>

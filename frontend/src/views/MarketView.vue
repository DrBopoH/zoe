<script setup>
import { computed, onMounted, watch } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import BondsView from './BondsView.vue'
import MacroRatesView from './MacroRatesView.vue'
import MacroInflationView from './MacroInflationView.vue'
import MacroFxView from './MacroFxView.vue'
import MacroUsView from './MacroUsView.vue'
import { useSection } from '../composables/useSection.js'
import { useMacro } from '../composables/useMacro.js'
import { toLocalDateStr } from '../utils/formatters.js'
import { FX_IDS, US_IDS } from '../utils/macro.js'
import { MARKET_TREE, MARKET_SECTIONS, isBondSection } from '../utils/marketNav.js'

// Оболочка вкладки «Рынок»: шторка с группами («Ценные бумаги», «Макро») и экран выбранного раздела.
// Макро-ряды читаются один раз здесь и раздаются экранам; при записи в кеш (dataVersion) перечитываются.
const props = defineProps({
  accounts: { type: Array, default: () => [] },
  dataVersion: { default: null },
  running: { type: Boolean, default: false }
})

const section = useSection('economic_market_section', MARKET_SECTIONS, 'nbu')
const macro = useMacro()
// Курсы и США тяжёлые (тысячи точек): читаются только на своих экранах
const fx = useMacro(FX_IDS)
const us = useMacro(US_IDS)
const heavy = { fx, fed: us }
onMounted(() => { macro.load(); heavy[section.value]?.load() })
watch(() => props.dataVersion, () => { macro.load(); heavy[section.value]?.load() })
watch(section, (s) => { const h = heavy[s]; if (h && !Object.keys(h.series.value).length) h.load() })
const today = computed(() => toLocalDateStr(new Date()))
const bondSection = computed(() => (isBondSection(section.value) ? section.value : null))
</script>

<template>
  <div class="relative space-y-4">
    <SectionRail v-model="section" :sections="MARKET_TREE" label="Разделы рынка" groups-key="economic_market_groups" />
    <KeepAlive>
      <BondsView v-if="bondSection" key="bonds" :section="bondSection" :accounts="accounts" :data-version="dataVersion" :running="running" :macro="macro.series.value" @go="section = $event" />
    </KeepAlive>
    <MacroRatesView v-if="section === 'rates'" :series="macro.series.value" :today="today" :running="running" />
    <MacroInflationView v-else-if="section === 'inflation'" :series="macro.series.value" :today="today" :running="running" />
    <MacroFxView v-else-if="section === 'fx'" :fx="fx.series.value" :macro="macro.series.value" :today="today" :running="running" />
    <MacroUsView v-else-if="section === 'fed'" :us="us.series.value" :macro="macro.series.value" :today="today" :running="running" />
  </div>
</template>

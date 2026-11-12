<script setup>
import { computed, onMounted, watch } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import BondsView from './BondsView.vue'
import MacroRatesView from './MacroRatesView.vue'
import MacroInflationView from './MacroInflationView.vue'
import { useSection } from '../composables/useSection.js'
import { useMacro } from '../composables/useMacro.js'
import { toLocalDateStr } from '../utils/formatters.js'
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
onMounted(() => macro.load())
watch(() => props.dataVersion, () => macro.load())
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
  </div>
</template>

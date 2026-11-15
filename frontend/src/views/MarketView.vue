<script setup>
import { computed, onMounted, watch } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import BondsView from './BondsView.vue'
import MacroRatesView from './MacroRatesView.vue'
import MacroInflationView from './MacroInflationView.vue'
import MacroFxView from './MacroFxView.vue'
import MacroCountriesView from './MacroCountriesView.vue'
import MacroEurozoneView from './MacroEurozoneView.vue'
import MacroUsView from './MacroUsView.vue'
import MacroUsEconView from './MacroUsEconView.vue'
import MacroUsYieldsView from './MacroUsYieldsView.vue'
import MacroUsRealView from './MacroUsRealView.vue'
import MacroUsPricesView from './MacroUsPricesView.vue'
import MacroUsJobsView from './MacroUsJobsView.vue'
import MacroUsActivityView from './MacroUsActivityView.vue'
import MacroUsLiquidityView from './MacroUsLiquidityView.vue'
import MacroUsCreditView from './MacroUsCreditView.vue'
import MacroCryptoView from './MacroCryptoView.vue'
import MacroStocksView from './MacroStocksView.vue'
import MacroCommoditiesView from './MacroCommoditiesView.vue'
import MacroWorldView from './MacroWorldView.vue'
import MacroChainView from './MacroChainView.vue'
import MacroPppView from './MacroPppView.vue'
import MacroScorecardView from './MacroScorecardView.vue'
import { useSection } from '../composables/useSection.js'
import { useMacro } from '../composables/useMacro.js'
import { useCountries } from '../composables/useCountries.js'
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
// Курсы и США тяжёлые (тысячи точек): читаются только на своих экранах
const fx = useMacro('fx')
const us = useMacro('us')
const econ = useMacro('usecon')
// Кривая: 10 лет лежит в источнике «us», поэтому берётся сверх рядов источника «uscurve»
const curve = useMacro('uscurve', ['ust_10y'])
const real = useMacro('usreal')
// Цены и труд сравниваются с инфляцией и безработицей, которые лежат в других источниках: берутся сверх своих рядов
const prices = useMacro('usprices', ['us_cpi_yoy'])
const jobs = useMacro('usjobs', ['us_unrate', 'us_cpi_yoy'])
const activity = useMacro('usact')
const liquidity = useMacro('usliq')
const credit = useMacro('uscredit')
// Еврозона: общая инфляция HICP лежит в источнике «macro», поэтому берётся сверх рядов «ez». Страны: ряды и список стран приходят с сервера
const ez = useMacro('ez', ['eu_cpi_yoy'])
const countries = useCountries()
const crypto = useMacro('crypto')
const stocks = useMacro('stocks')
// Сырьё: цены лежат в источнике «commodities», металлы и курс доллара — в «fx», поэтому берутся сверх рядов источника
const world = useMacro('world')
// Крипто-контур сравнивает P2P с официальным курсом доллара (источник «fx»)
const chain = useMacro('chain', ['usd_uah'])
const ppp = useMacro('ppp')
const score = useMacro('usecon', ['fed_funds', 'us_cpi_yoy', 'us_pce_yoy', 'us_corecpi_yoy', 'us_claims', 'us_hy', 'us_nfci', 'us_usd'])
const scoreScreen = { series: score.series, load: () => Promise.all([score.load(), countries.load()]) }
const commodities = useMacro('commodities', ['usd_uah', 'xau_uah', 'xag_uah', 'xpt_uah', 'xpd_uah'])
// «Курсы» читают ещё и реестр стран (ставки и реальные курсы в таблице мировых валют)
const fxScreen = { series: fx.series, load: () => Promise.all([fx.load(), countries.load()]) }
const heavy = { countries, eurozone: ez, fx: fxScreen, fed: us, yields: curve, real, prices, jobs, activity, liquidity, credit, usecon: econ, crypto, stocks, commodities, world, chain, ppp, scorecard: scoreScreen }
// Таблицы облигаций считают доходность в долларах: нужны курсы и инфляция США (тяжёлые ряды, поэтому только на этих экранах)
const dollar = computed(() => ({ ...fx.series.value, ...us.series.value }))
const loadBond = () => { if (!bondSection.value) return; for (const h of [fx, us]) if (!Object.keys(h.series.value).length) h.load() }
onMounted(() => { macro.load(); heavy[section.value]?.load(); loadBond() })
watch(() => props.dataVersion, () => { macro.load(); heavy[section.value]?.load(); if (bondSection.value) { fx.load(); us.load() } })
watch(section, (s) => { const h = heavy[s]; if (h && !Object.keys(h.series.value).length) h.load(); loadBond() })
const today = computed(() => toLocalDateStr(new Date()))
const bondSection = computed(() => (isBondSection(section.value) ? section.value : null))
</script>

<template>
  <div class="relative space-y-4">
    <SectionRail v-model="section" :sections="MARKET_TREE" label="Разделы рынка" groups-key="economic_market_groups" />
    <KeepAlive>
      <BondsView v-if="bondSection" key="bonds" :section="bondSection" :accounts="accounts" :data-version="dataVersion" :running="running" :macro="macro.series.value" :dollar="dollar" @go="section = $event" />
    </KeepAlive>
    <MacroRatesView v-if="section === 'rates'" :series="macro.series.value" :today="today" :running="running" />
    <MacroInflationView v-else-if="section === 'inflation'" :series="macro.series.value" :today="today" :running="running" />
    <MacroFxView v-else-if="section === 'fx'" :fx="fx.series.value" :macro="macro.series.value" :countries="countries.countries.value" :cseries="countries.series.value" :today="today" :running="running" />
    <MacroCountriesView v-else-if="section === 'countries'" :countries="countries.countries.value" :series="countries.series.value" :today="today" :running="running" />
    <MacroEurozoneView v-else-if="section === 'eurozone'" :ez="ez.series.value" :today="today" :running="running" />
    <MacroUsView v-else-if="section === 'fed'" :us="us.series.value" :macro="macro.series.value" :today="today" :running="running" />
    <MacroUsYieldsView v-else-if="section === 'yields'" :curve="curve.series.value" :today="today" :running="running" />
    <MacroUsRealView v-else-if="section === 'real'" :real="real.series.value" :today="today" :running="running" />
    <MacroUsPricesView v-else-if="section === 'prices'" :prices="prices.series.value" :today="today" :running="running" />
    <MacroUsJobsView v-else-if="section === 'jobs'" :jobs="jobs.series.value" :today="today" :running="running" />
    <MacroUsActivityView v-else-if="section === 'activity'" :activity="activity.series.value" :today="today" :running="running" />
    <MacroUsLiquidityView v-else-if="section === 'liquidity'" :liquidity="liquidity.series.value" :today="today" :running="running" />
    <MacroUsCreditView v-else-if="section === 'credit'" :credit="credit.series.value" :today="today" :running="running" />
    <MacroUsEconView v-else-if="section === 'usecon'" :econ="econ.series.value" :today="today" :running="running" />
    <MacroStocksView v-else-if="section === 'stocks'" :stocks="stocks.series.value" :today="today" :running="running" />
    <MacroCommoditiesView v-else-if="section === 'commodities'" :com="commodities.series.value" :today="today" :running="running" />
    <MacroWorldView v-else-if="section === 'world'" :world="world.series.value" :today="today" :running="running" />
    <MacroChainView v-else-if="section === 'chain'" :chain="chain.series.value" :today="today" :running="running" />
    <MacroScorecardView v-else-if="section === 'scorecard'" :econ="score.series.value" :countries="countries.countries.value" :cseries="countries.series.value" :today="today" :running="running" />
    <MacroPppView v-else-if="section === 'ppp'" :ppp="ppp.series.value" :macro="macro.series.value" :today="today" :running="running" />
    <MacroCryptoView v-else-if="section === 'crypto'" :crypto="crypto.series.value" :today="today" :running="running" />
  </div>
</template>

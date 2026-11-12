<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { RANGES, macroSummary, signedPct, plainPct } from '../utils/macro.js'

// Экран «Ставка НБУ»: облікова ставка, овернайт и реальная ставка с поправкой на последнюю инфляцию.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh, startError } = useMarketRefresh(() => 'rates')
const IDS = ['policy_rate', 'overnight_cd', 'overnight_loans']
const sum = computed(() => macroSummary(props.series, props.today))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Ставка НБУ</h2>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>
    <div v-if="startError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError }}</div>
    <MacroPanel title="Ключевые ставки НБУ" :series="series" :today="today" :ids="IDS" :ranges="RANGES" default-range="10">
      <template #summary>
        <span class="text-slate-400">Облікова ставка <b class="text-emerald-400 tabular-nums">{{ plainPct(sum.policy) }}</b></span>
        <span v-if="sum.overnightCD !== null" class="text-slate-400" title="Ставка по депозитным сертификатам овернайт">Сертификаты овернайт <b class="text-sky-300 tabular-nums">{{ plainPct(sum.overnightCD) }}</b></span>
        <span v-if="sum.realPolicy !== null" class="text-slate-400" title="(1 + ставка) / (1 + инфляция) − 1, инфляция г/г за последний известный месяц">Реальная ставка <b class="tabular-nums" :class="sum.realPolicy >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(sum.realPolicy) }}</b></span>
      </template>
    </MacroPanel>
  </div>
</template>

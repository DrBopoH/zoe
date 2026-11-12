<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { RANGES, macroSummary, plainPct } from '../utils/macro.js'

// Экран «Инфляция»: год к году (общая и базовая) и месяц к месяцу — два графика с общим движком.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh, startError } = useMarketRefresh(() => 'inflation')
const YOY = ['cpi_yoy', 'core_yoy']
const MOM = ['cpi_mom', 'core_mom']
const sum = computed(() => macroSummary(props.series, props.today))
const month = computed(() => (sum.value.cpiMonth ? `${sum.value.cpiMonth.slice(5, 7)}.${sum.value.cpiMonth.slice(0, 4)}` : ''))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Инфляция</h2>
      <span v-if="month" class="text-slate-500">последняя публикация: {{ month }}</span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>
    <div v-if="startError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError }}</div>
    <MacroPanel title="Инфляция к тому же месяцу прошлого года" :series="series" :today="today" :ids="YOY" :ranges="RANGES" default-range="2007">
      <template #summary>
        <span class="text-slate-400">Инфляция г/г <b class="text-amber-300 tabular-nums">{{ plainPct(sum.cpi) }}</b></span>
        <span class="text-slate-400">Базовая г/г <b class="text-orange-300 tabular-nums">{{ plainPct(sum.core) }}</b></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Инфляция к предыдущему месяцу" :series="series" :today="today" :ids="MOM" :ranges="RANGES" default-range="5" hint="Месячные скачки: тот же масштаб и сдвиг, что у годового графика" />
  </div>
</template>

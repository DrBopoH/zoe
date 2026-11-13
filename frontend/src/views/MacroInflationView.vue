<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { macroSummary, plainPct, lastPoint } from '../utils/macro.js'

// Экран «Инфляция»: год к году (общая и базовая) и месяц к месяцу — два графика с общим движком.
const props = defineProps({
  series: { type: Object, default: () => ({}) },
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'inflation')
const YOY = ['cpi_yoy', 'core_yoy']
const MOM = ['cpi_mom', 'core_mom']
const sum = computed(() => macroSummary(props.series, props.today))
const eu = computed(() => lastPoint(props.series.eu_cpi_yoy))
const euMonth = computed(() => (eu.value ? `${eu.value.d.slice(5, 7)}.${eu.value.d.slice(0, 4)}` : ''))
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
    <MacroPanel title="Инфляция к тому же месяцу прошлого года" :series="series" :today="today" :ids="YOY">
      <template #summary>
        <span class="text-slate-400">Инфляция г/г <b class="text-amber-300 tabular-nums">{{ plainPct(sum.cpi) }}</b></span>
        <span class="text-slate-400">Базовая г/г <b class="text-orange-300 tabular-nums">{{ plainPct(sum.core) }}</b></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Инфляция еврозоны (HICP) к тому же месяцу прошлого года" :series="series" :today="today" :ids="['eu_cpi_yoy']" empty="Нет данных. Нажми «Обновить»: подтянется инфляция еврозоны с Евростата" hint="Колесо: масштаб, перетаскивание: сдвиг. Нужна для реальной доходности облигаций в евро">
      <template #summary>
        <span v-if="eu" class="text-slate-400">Еврозона г/г <b class="text-sky-300 tabular-nums">{{ plainPct(eu.v) }}</b><span class="text-slate-600"> · {{ euMonth }}</span></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Инфляция к предыдущему месяцу" :series="series" :today="today" :ids="MOM" hint="Месячные скачки: тот же масштаб и сдвиг, что у годового графика" />
  </div>
</template>

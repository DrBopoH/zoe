<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { RANGES, lastPoint, valueAt, realRate, signedPct, plainPct } from '../utils/macro.js'

// Экран «Ставка ФРС»: ставка ФРС, доходность 10-летних Treasury и инфляция США (FRED) — рядом с украинскими, чтобы сравнивать.
const props = defineProps({
  us: { type: Object, default: () => ({}) }, // fed_funds, fed_target, ust_10y, us_cpi_yoy, us_cpi_mom
  macro: { type: Object, default: () => ({}) }, // украинские ряды: облікова и инфляция для сравнения
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh, startError } = useMarketRefresh(() => 'fed')
const RATES = ['fed_funds', 'fed_target', 'ust_10y']
const YOY = ['us_cpi_yoy']
const MOM = ['us_cpi_mom']

const at = (id) => valueAt(props.us[id], props.today)
const target = computed(() => at('fed_target'))
const effective = computed(() => lastPoint(props.us.fed_funds)?.v ?? null)
const t10 = computed(() => lastPoint(props.us.ust_10y)?.v ?? null)
const cpi = computed(() => lastPoint(props.us.us_cpi_yoy))
const month = computed(() => (cpi.value ? `${cpi.value.d.slice(5, 7)}.${cpi.value.d.slice(0, 4)}` : ''))
const real = computed(() => (target.value !== null && cpi.value ? realRate(target.value, cpi.value.v) : null))
const uaPolicy = computed(() => valueAt(props.macro.policy_rate, props.today))
const uaCpi = computed(() => lastPoint(props.macro.cpi_yoy)?.v ?? null)
const uaReal = computed(() => (uaPolicy.value !== null && uaCpi.value !== null ? realRate(uaPolicy.value, uaCpi.value) : null))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Ставка ФРС</h2>
      <span v-if="month" class="text-slate-500">инфляция США: последняя публикация {{ month }}</span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>
    <div v-if="startError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError }}</div>

    <MacroPanel title="Ставка ФРС и доходность 10-летних Treasury" :series="us" :today="today" :ids="RATES" :ranges="RANGES" default-range="10" empty="Нет данных. Нажми «Обновить»: подтянутся ряды США из FRED" hint="Колесо: масштаб, перетаскивание: сдвиг, двойной клик: сброс. Ставка ФРС — эффективная по месяцам; ступенька — верхняя граница целевого коридора с 2008 года">
      <template #summary>
        <span v-if="target !== null" class="text-slate-400" title="Верхняя граница целевого диапазона ставки ФРС">ФРС (верх коридора) <b class="text-emerald-400 tabular-nums">{{ plainPct(target) }}</b></span>
        <span v-if="effective !== null" class="text-slate-400">Эффективная <b class="text-violet-300 tabular-nums">{{ plainPct(effective) }}</b></span>
        <span v-if="t10 !== null" class="text-slate-400" title="Доходность 10-летних облигаций США: ориентир для долларовых бумаг">Treasury 10 л. <b class="text-sky-300 tabular-nums">{{ plainPct(t10) }}</b></span>
        <span v-if="real !== null" class="text-slate-400" title="(1 + ставка) / (1 + инфляция) − 1">Реальная ставка ФРС <b class="tabular-nums" :class="real >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(real) }}</b></span>
      </template>
    </MacroPanel>

    <section v-if="uaReal !== null && real !== null" class="text-xs bg-slate-900/60 border border-slate-800 rounded px-4 py-3 space-y-1">
      <div class="text-[10px] uppercase tracking-wider font-bold text-slate-500">Украина и США рядом</div>
      <div class="text-slate-400">Облікова <b class="text-emerald-400 tabular-nums">{{ plainPct(uaPolicy) }}</b> при инфляции {{ plainPct(uaCpi) }}: реальная <b class="tabular-nums" :class="uaReal >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(uaReal) }}</b>. ФРС <b class="text-emerald-400 tabular-nums">{{ plainPct(target) }}</b> при инфляции {{ plainPct(cpi.v) }}: реальная <b class="tabular-nums" :class="real >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(real) }}</b>.</div>
      <div class="text-slate-600">Разница реальных ставок объясняет, почему гривневые бумаги платят больше долларовых, и сколько из этого съедает девальвация (экран «Курсы»).</div>
    </section>

    <MacroPanel title="Инфляция США к тому же месяцу прошлого года" :series="us" :today="today" :ids="YOY" :ranges="RANGES" default-range="10" empty="Нет данных. Нажми «Обновить»: подтянутся ряды США из FRED">
      <template #summary>
        <span v-if="cpi" class="text-slate-400">Инфляция США г/г <b class="text-amber-300 tabular-nums">{{ plainPct(cpi.v) }}</b></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Инфляция США к предыдущему месяцу" :series="us" :today="today" :ids="MOM" :ranges="RANGES" default-range="5" empty="Нет данных. Нажми «Обновить»: подтянутся ряды США из FRED" />
  </div>
</template>

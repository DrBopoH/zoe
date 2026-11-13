<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, sahmRule, technicalRecession, curveState, vixLevel } from '../utils/macro.js'

// Экран «Экономика США»: рынок труда, рост, кривая доходности, страх и настроения (FRED). Это не сигналы к действию, а картина:
// подсказки ниже графиков — как исторически читают эти ряды.
const props = defineProps({
  econ: { type: Object, default: () => ({}) }, // us_unrate, us_gdp, us_spread, us_vix, us_sentiment
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh, startError } = useMarketRefresh(() => 'usecon')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянется экономика США из FRED'
const monthOf = (p) => (p ? `${p.d.slice(5, 7)}.${p.d.slice(0, 4)}` : '')
const quarterOf = (p) => (p ? `${Math.floor((Number(p.d.slice(5, 7)) - 1) / 3) + 1} кв. ${p.d.slice(0, 4)}` : '')

const unrate = computed(() => lastPoint(props.econ.us_unrate))
const sahm = computed(() => sahmRule(props.econ.us_unrate))
const gdp = computed(() => lastPoint(props.econ.us_gdp))
const techRec = computed(() => technicalRecession(props.econ.us_gdp))
const spread = computed(() => lastPoint(props.econ.us_spread))
const curve = computed(() => curveState(spread.value?.v))
const vix = computed(() => lastPoint(props.econ.us_vix))
// Цвет числа как у линии: самоуспокоенность (ниже 14) янтарная, обычный фон серый, тревога и паника голубые
const vixColor = computed(() => { const x = vix.value?.v; return x < 14 ? 'text-amber-300' : x <= 20 ? 'text-slate-300' : 'text-sky-300' })
const vixWord = computed(() => ({ calm: 'спокойно', nervous: 'тревожно', panic: 'паника' })[vixLevel(vix.value?.v)] || '')
const sentiment = computed(() => lastPoint(props.econ.us_sentiment))
// Цвет числа как у линии: ниже 65 красный, до 85 жёлтый, выше зелёный
const sentimentColor = computed(() => { const x = sentiment.value?.v; return x < 65 ? 'text-rose-400' : x <= 85 ? 'text-amber-300' : 'text-emerald-400' })
const CURVE_TEXT = { inverted: 'инвертирована: короткие ставки выше длинных, исторически это опережало рецессии', flat: 'почти плоская', normal: 'нормальная: длинные ставки выше коротких' }
const has = computed(() => Object.values(props.econ).some((a) => a && a.length))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Экономика США</h2>
      <span v-if="unrate" class="text-slate-500">рынок труда: последняя публикация {{ monthOf(unrate) }}</span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>
    <div v-if="startError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError }}</div>

    <MacroPanel title="Безработица США" :series="econ" :today="today" :ids="['us_unrate']" :empty="EMPTY">
      <template #summary>
        <span v-if="unrate" class="text-slate-400">Сейчас <b class="text-sky-300 tabular-nums">{{ plainPct(unrate.v) }}</b></span>
        <span v-if="sahm" class="text-slate-400" title="Средняя безработица за 3 месяца минус её минимум за 12 месяцев. Рост на 0,5 п. п. и выше исторически совпадал с началом рецессии">Правило Сам <b class="tabular-nums" :class="sahm.signal ? 'text-rose-400' : 'text-emerald-400'">{{ String(sahm.value).replace('.', ',') }} п. п.</b><span class="text-slate-600"> · {{ sahm.signal ? 'сигнал рецессии' : 'сигнала нет' }}</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Рост ВВП США (реальный, в пересчёте на год, поквартально)" :series="econ" :today="today" :ids="['us_gdp']" :empty="EMPTY">
      <template #summary>
        <span v-if="gdp" class="text-slate-400">{{ quarterOf(gdp) }} <b class="tabular-nums" :class="gdp.v >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ plainPct(gdp.v) }}</b></span>
        <span v-if="techRec" class="text-rose-400">два квартала подряд спада: техническая рецессия</span>
      </template>
    </MacroPanel>

    <MacroPanel title="Кривая доходности: 10-летние минус 2-летние Treasury" unit="п. п." :series="econ" :today="today" :ids="['us_spread']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Ниже нуля кривая «инвертирована»">
      <template #summary>
        <span v-if="spread" class="text-slate-400">Сейчас <b class="tabular-nums" :class="spread.v < 0 ? 'text-rose-400' : 'text-emerald-400'">{{ String(Math.round(spread.v * 100) / 100).replace('.', ',') }} п. п.</b></span>
        <span v-if="curve" class="text-slate-500">кривая {{ CURVE_TEXT[curve] }}</span>
      </template>
    </MacroPanel>

    <MacroPanel title="VIX: индекс страха рынка акций" unit="п." :series="econ" :today="today" :ids="['us_vix']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. До 20 спокойно, 20–30 тревожно, выше 30 паника">
      <template #summary>
        <span v-if="vix" class="text-slate-400">Сейчас <b class="tabular-nums" :class="vixColor">{{ String(Math.round(vix.v * 10) / 10).replace('.', ',') }}</b><span class="text-slate-600"> · {{ vixWord }}</span></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Настроения потребителей (Мичиган)" unit="п." :series="econ" :today="today" :ids="['us_sentiment']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. Индекс: чем выше, тем оптимистичнее люди">
      <template #summary>
        <span v-if="sentiment" class="text-slate-400">{{ monthOf(sentiment) }} <b class="tabular-nums" :class="sentimentColor">{{ String(Math.round(sentiment.v * 10) / 10).replace('.', ',') }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Ряды FRED (Федеральный резервный банк Сент-Луиса). Подсказки — как эти показатели читают исторически, а не прогноз и не совет.</div>
  </div>
</template>

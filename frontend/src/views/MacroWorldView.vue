<script setup>
import { numText } from '../utils/numfmt.js'
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, signedPct, plainPct, unitText, hasPoints, dateOf } from '../utils/macro.js'
import { WORLD_GROUPS, worldIds, worldRows, buffettLevel } from '../utils/worldmarkets.js'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'

// Экран «Мировые акции»: индексы ОЭСР по странам (помесячно, 2015 = 100) и оценка рынка США: индекс Баффета, CAPE и дивидендная доходность.
// CAPE и дивиденды — таблица Шиллера, она не обновляется с 2023 года: это история, а не текущая оценка.
// Не прогноз и не совет: где рынок относительно своих пиков и относительно истории.
const props = defineProps({
  world: { type: Object, default: () => ({}) }, // wm_<cc>, val_buffett, val_cape, val_divy
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'world')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся индексы акций стран и оценка рынка США'
const rows = computed(() => worldRows(props.world, props.today))
const has = computed(() => hasPoints(props.world))
const lastDate = computed(() => rows.value.map((r) => r.last?.d).filter(Boolean).sort().pop() || null)
const buf = computed(() => lastPoint(props.world.val_buffett))
const cape = computed(() => lastPoint(props.world.val_cape))
const divy = computed(() => lastPoint(props.world.val_divy))
const chg = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
const num = (v) => numText(v, 1)
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Мировые акции" :running="running" @refresh="refresh">
      <span v-if="lastDate" class="text-slate-500">помесячно · последняя точка {{ dateOf(lastDate) }}</span>
    </MarketHeader>

    <UiCard class="overflow-x-auto">
      <table v-if="has" class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr><th class="px-3 py-1.5 text-left font-bold">Страна</th><th class="px-2 text-right font-bold">Индекс</th><th class="px-2 text-right font-bold">За год</th><th class="px-3 text-right font-bold">От максимума</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="r in rows" :key="r.id">
            <td class="px-3 py-1.5 text-slate-300">{{ r.label }}</td>
            <td class="px-2 text-right text-slate-100">{{ r.last ? num(r.last.v) : '—' }}</td>
            <td class="px-2 text-right" :class="chg(r.chg1y)">{{ r.chg1y === null ? '—' : signedPct(r.chg1y) }}</td>
            <td class="px-3 text-right" :class="chg(r.fromAth === null ? null : r.fromAth >= -1 ? 1 : -1)">{{ r.fromAth === null ? '—' : plainPct(r.fromAth) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>{{ EMPTY }}</UiEmpty>
    </UiCard>

    <MacroPanel v-for="g in WORLD_GROUPS" :key="g.key" :title="`${g.title}, индекс (2015 = 100)`" unit="п." :series="world" :today="today" :ids="worldIds(g.key)" :log="!!g.log" :hint="g.hint" :empty="EMPTY" />

    <MacroPanel title="Индекс Баффета: стоимость акций США к ВВП, %" unit="%" :series="world" :today="today" :ids="['val_buffett']" :empty="EMPTY" hint="Рыночная стоимость акций американских компаний к ВВП, поквартально. Чем выше, тем дороже рынок относительно экономики; это не сигнал к действию">
      <template #summary>
        <span v-if="buf" class="text-slate-400">Сейчас <b class="text-amber-300 tabular-nums">{{ plainPct(buf.v) }}</b> · {{ buffettLevel(buf.v) }}</span>
      </template>
    </MacroPanel>

    <MacroPanel title="CAPE S&P 500 (цена к средней прибыли за 10 лет)" unit="раз" :series="world" :today="today" :ids="['val_cape']" :empty="EMPTY" :hint="`Данные Шиллера кончаются в ${cape ? dateOf(cape.d) : '2023'}: дальше источник не обновляется`">
      <template #summary>
        <span v-if="cape" class="text-slate-400">Последняя точка <b class="text-violet-300 tabular-nums">{{ unitText(cape.v, 'раз') }}</b> · {{ dateOf(cape.d) }}</span>
      </template>
    </MacroPanel>

    <MacroPanel title="Дивидендная доходность S&P 500, %" unit="%" :series="world" :today="today" :ids="['val_divy']" :empty="EMPTY" :hint="`Данные Шиллера кончаются в ${divy ? dateOf(divy.d) : '2023'}: дальше источник не обновляется`">
      <template #summary>
        <span v-if="divy" class="text-slate-400">Последняя точка <b class="text-emerald-400 tabular-nums">{{ plainPct(divy.v) }}</b> · {{ dateOf(divy.d) }}</span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Данные: индексы акций ОЭСР через FRED (помесячно, 2015 = 100, местная валюта), стоимость акций и ВВП США — FRED, CAPE и дивиденды — таблица Роберта Шиллера (до 2023 года). Это картина рынка, а не прогноз и не совет.</div>
  </div>
</template>

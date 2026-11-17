<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'
import UiLoadError from '../components/ui/UiLoadError.vue'
import UiNotice from '../components/ui/UiNotice.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { useStocks } from '../composables/useStocks.js'
import { dateOf, signedPct } from '../utils/macro.js'
import { numText } from '../utils/numfmt.js'
import { stockSeries, stockRatios, capCheck, usdText, ratioText } from '../utils/shares.js'
import { billionsText } from '../utils/macro.js'

// Экран «Акции компаний»: список компаний США по капитализации (Nasdaq + SEC), поиск по тикеру и названию, а для выбранной — цена (Yahoo, помесячно),
// капитализация во времени (цена × число акций из отчётности SEC, приведённое по сплитам), выручка и прибыль за год, капитал и активы (SEC EDGAR).
// Истории цен и отчётность качаются для топ-500 в фоне («Обновить»), остальные компании — по первому открытию. Не прогноз и не совет.
const props = defineProps({
  today: { type: String, required: true },
  running: { type: Boolean, default: false },
  dataVersion: { default: null }
})
const { refresh } = useMarketRefresh(() => 'shares')
const stocks = useStocks()
const query = ref('')
const ticker = ref('')
let timer = null

const pick = (t) => { ticker.value = t; stocks.open(t) }
const reload = async () => {
  await stocks.search(query.value)
  if (!ticker.value && stocks.list.value.length) pick(stocks.list.value[0].ticker)
  else if (ticker.value) stocks.open(ticker.value)
}
onMounted(reload)
// Опрос закончился (запись в кеш): перечитать список и открытую компанию
watch(() => props.dataVersion, reload)
// Поиск печатается быстрее, чем отвечает сервер: небольшая пауза, ответы вразнобой отсекает composable
watch(query, (v) => { clearTimeout(timer); timer = setTimeout(() => stocks.search(v), 250) })
onBeforeUnmount(() => clearTimeout(timer))

const co = computed(() => stocks.detail.value?.company || null)
const series = computed(() => stockSeries(stocks.detail.value))
const ratios = computed(() => stockRatios(stocks.detail.value, props.today))
const check = computed(() => (co.value ? capCheck(co.value.market_cap, stocks.detail.value.cap, props.today) : null))
const hasCap = computed(() => series.value.sh_cap.length > 0)
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся список компаний, цены и отчётность SEC'
const money = (p) => (p ? usdText(p.v) : '—')
const change = (a, months) => {
  const n = a?.length || 0
  return n > months && a[n - 1 - months].v > 0 ? (a[n - 1].v / a[n - 1 - months].v - 1) * 100 : null
}
const y1 = computed(() => change(series.value.sh_price, 12))
const y5 = computed(() => change(series.value.sh_price, 60))
const tone = (v) => (v === null ? 'text-slate-500' : v >= 0 ? 'text-emerald-400' : 'text-rose-400')
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Акции компаний" :running="running" @refresh="refresh">
      <input v-model="query" type="text" class="ui-input !bg-slate-900 !w-56" placeholder="Поиск: тикер или название" />
      <span v-if="co" class="text-slate-500">{{ co.exchange }}<template v-if="co.sector"> · {{ co.sector }}</template><template v-if="co.industry"> · {{ co.industry }}</template></span>
    </MarketHeader>

    <UiLoadError v-if="stocks.error.value" :message="stocks.error.value" @retry="reload" />

    <UiCard class="max-h-72 overflow-auto">
      <table v-if="stocks.list.value.length" class="w-full text-xs tabular-nums">
        <thead class="sticky top-0 bg-slate-900 text-[10px] uppercase tracking-wider text-slate-500">
          <tr><th class="px-3 py-1.5 text-left font-bold">Тикер</th><th class="px-2 text-left font-bold">Компания</th><th class="px-2 text-right font-bold">Цена</th><th class="px-3 text-right font-bold">Капитализация (Nasdaq)</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="c in stocks.list.value" :key="c.ticker" class="cursor-pointer hover:bg-slate-800/40" :class="c.ticker === ticker ? 'bg-slate-800/60' : ''" @click="pick(c.ticker)">
            <td class="px-3 py-1.5 font-bold text-slate-200">{{ c.ticker }}</td>
            <td class="px-2 text-slate-300">{{ c.name }}</td>
            <td class="px-2 text-right text-slate-100">{{ c.price > 0 ? `${numText(c.price, 2)} $` : '—' }}</td>
            <td class="px-3 text-right text-slate-100">{{ usdText(c.market_cap) }}</td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>{{ query ? 'Ничего не найдено' : EMPTY }}</UiEmpty>
    </UiCard>

    <UiNotice v-if="check && !check.ok">
      Расчётная капитализация {{ billionsText(check.own) }} расходится с Nasdaq ({{ billionsText(check.nasdaq) }}) на {{ numText(Math.abs(check.diff), 0) }} %. Обычно так бывает, когда у компании несколько классов акций
      (например GOOGL, BRK) и в отчёте SEC указан только один из них; верить стоит цифре Nasdaq.
    </UiNotice>

    <template v-if="co">
      <MacroPanel :title="`${co.ticker} · ${co.name}: цена закрытия месяца, $`" unit="$" :series="series" :today="today" :ids="['sh_price']" log benner :empty="EMPTY" hint="Цена поправлена на сплиты. Колесо: масштаб, перетаскивание: сдвиг">
        <template #summary>
          <span v-if="co.price > 0" class="text-slate-400">Сейчас <b class="text-sky-300 tabular-nums">{{ numText(co.price, 2) }} $</b></span>
          <span v-if="y1 !== null" class="text-slate-400">За год <b class="tabular-nums" :class="tone(y1)">{{ signedPct(y1) }}</b></span>
          <span v-if="y5 !== null" class="text-slate-400">За 5 лет <b class="tabular-nums" :class="tone(y5)">{{ signedPct(y5) }}</b></span>
        </template>
      </MacroPanel>

      <MacroPanel title="Капитализация: цена × число акций из отчётности SEC, млрд $" unit="млрд" :series="series" :today="today" :ids="['sh_cap']" log benner :empty="hasCap ? EMPTY : 'Нет расчёта: у компании нет числа акций в отчётности SEC или оно ещё не скачано (кнопка «Обновить»)'" hint="Число акций приведено к сегодняшней базе по сплитам; месяцы без свежего отчёта пропущены">
        <template #summary>
          <span v-if="ratios" class="text-slate-400">По отчётам <b class="text-emerald-400 tabular-nums">{{ usdText(ratios.cap) }}</b> · {{ dateOf(ratios.capDate) }}</span>
          <span v-if="co.market_cap > 0" class="text-slate-400">Nasdaq <b class="text-slate-200 tabular-nums">{{ usdText(co.market_cap) }}</b></span>
        </template>
      </MacroPanel>

      <MacroPanel title="Выручка и чистая прибыль за год, млрд $" unit="млрд" :series="series" :today="today" :ids="['sh_revenue', 'sh_income']" :empty="'Нет отчётности: годовые данные SEC ещё не скачаны (кнопка «Обновить») или у компании их нет'" hint="Год у каждой компании свой (финансовый), значение действует до следующего отчёта">
        <template #summary>
          <span class="text-slate-400">Выручка <b class="text-amber-300 tabular-nums">{{ money(ratios?.revenue) }}</b></span>
          <span class="text-slate-400">Прибыль <b class="tabular-nums" :class="ratios?.income && ratios.income.v < 0 ? 'text-rose-400' : 'text-violet-300'">{{ money(ratios?.income) }}</b></span>
          <span class="text-slate-400" title="Капитализация / выручка за последний год">P/S <b class="text-slate-200 tabular-nums">{{ ratioText(ratios?.ps) }}</b></span>
          <span class="text-slate-400" title="Капитализация / чистая прибыль за последний год; при убытке не считается">P/E <b class="text-slate-200 tabular-nums">{{ ratioText(ratios?.pe) }}</b></span>
        </template>
      </MacroPanel>

      <MacroPanel title="Активы и капитал на конец квартала, млрд $" unit="млрд" :series="series" :today="today" :ids="['sh_assets', 'sh_equity']" :empty="'Нет отчётности: баланс SEC ещё не скачан (кнопка «Обновить») или у компании его нет'">
        <template #summary>
          <span class="text-slate-400">Капитал <b class="text-sky-300 tabular-nums">{{ money(ratios?.equity) }}</b></span>
          <span class="text-slate-400" title="Капитализация / собственный капитал; при отрицательном капитале не считается">P/B <b class="text-slate-200 tabular-nums">{{ ratioText(ratios?.pb) }}</b></span>
        </template>
      </MacroPanel>

      <div class="text-[11px] text-slate-600">Данные: список — Nasdaq и SEC, цены — Yahoo Finance (помесячно), отчётность — SEC EDGAR (XBRL, кадры по всем компаниям). Мультипликаторы считаются по последним годовым отчётам; для банков и компаний с нестандартной отчётностью выручка может отсутствовать. Это картина, а не прогноз и не совет.</div>
    </template>
    <UiEmpty v-else-if="!stocks.loading.value && !stocks.error.value">Выбери компанию в списке</UiEmpty>
  </div>
</template>

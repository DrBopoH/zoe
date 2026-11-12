<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import UiButton from '../components/ui/UiButton.vue'
import GrowthChart from '../components/GrowthChart.vue'
import { storageGet, storageSet } from '../utils/storage.js'
import IcuIntake from '../components/IcuIntake.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { useIcuList } from '../composables/useIcuList.js'
import { currentInflation, realRate, signedPct, plainPct, macroSummary } from '../utils/macro.js'
import { apiGetJson, apiSend, apiJson } from '../utils/api.js'
import { toLocalDateStr } from '../utils/formatters.js'
import { SOURCES, COLUMNS, filterBonds, sortBonds, currenciesOf, bondCells, nextSort, resolveRates, holdingsOf, qtyText, restrictTo, scoreBonds, scoreColor, HORIZON_YEARS, moneyText, isMatured, rateOf } from '../utils/marketTable.js'

// dataVersion растёт при любой записи в кеш (в том числе после опроса рынка) — по нему перечитываем список.
// running — идёт ли сейчас опрос рынка (из /sync/status → market.running).
const props = defineProps({
  accounts: { type: Array, default: () => [] }, // счета: из брокерских (ICU) берутся уже купленные бумаги
  dataVersion: { default: null },
  running: { type: Boolean, default: false },
  section: { type: String, default: 'nbu' }, // nbu | ust: какой реестр показывать (раздел выбирает оболочка вкладки)
  macro: { type: Object, default: () => ({}) } // макро-ряды НБУ (для колонки «Реальная» и полоски со ставкой)
})
const emit = defineEmits(['go'])

const section = computed(() => props.section)
const source = computed(() => SOURCES.find((s) => s.key === section.value) || SOURCES[0])

const lists = ref({}) // source → список; у каждого источника свой, чтобы возврат на вкладку не мигал пустотой
const loadError = ref('')
const { refresh, startError } = useMarketRefresh(() => section.value)

const load = async (src) => {
  try {
    lists.value = { ...lists.value, [src]: (await apiGetJson(`/market/bonds?source=${src}`)) || [] }
    loadError.value = ''
  } catch (e) {
    loadError.value = String(e.message || e)
  }
}
onMounted(() => { load(section.value); icu.load() })
watch(section, (s) => { if (!lists.value[s]) load(s) })
watch(() => props.dataVersion, () => load(section.value))

const q = ref('')
const currency = ref('')
const hideMatured = ref(true)
const sort = ref({ key: 'maturity', dir: 'asc' })
watch(section, () => { currency.value = '' }) // у другого источника нет такой валюты

const today = computed(() => toLocalDateStr(new Date()))
const list = computed(() => lists.value[section.value])
const currencies = computed(() => currenciesOf(list.value))
// Коэффициент и места считаются по всему списку источника, а не по найденному: поиск не меняет цвета и места
// «Только ICU»: свой список бумаг брокера (ISIN, цена и доходность с его экрана, кривая доходности); места и «Топ 1» считаются среди них.
const icu = useIcuList()
const onlyIcu = ref(storageGet('zoe.market.onlyIcu', '0') === '1')
const icuOpen = ref(false)
const icuSet = computed(() => new Set(icu.isins.value))
const icuActive = computed(() => section.value === 'nbu' && onlyIcu.value && icuSet.value.size > 0)
watch(onlyIcu, (v) => storageSet('zoe.market.onlyIcu', v ? '1' : '0'))
// Уже куплено (штук): позиции брокерских счетов; обновляется вместе с данными
const holdings = ref(new Map())
const loadHoldings = async () => {
  const accs = (props.accounts || []).filter((a) => a.bank_key === 'icu')
  try {
    holdings.value = holdingsOf(await Promise.all(accs.map((a) => apiGetJson(`/icu/ledger?account_id=${encodeURIComponent(a.id)}`))))
  } catch (e) {
    holdings.value = new Map()
  }
}
watch(() => [props.accounts, props.dataVersion], loadHoldings, { immediate: true })

const pool = computed(() => (icuActive.value ? restrictTo(list.value, icuSet.value) : list.value))
// Доходность по данным брокера (только в режиме «только ICU»): по цене, по показанной YTM или по кривой; без них — аукционная/купонная
const icuRates = computed(() => {
  const m = new Map()
  if (!icuActive.value) return m
  const r = resolveRates({ pool: pool.value, prices: icu.prices.value, ytms: icu.ytms.value, curve: icu.curve.value, today: today.value })
  for (const [id, v] of r) m.set(id, v)
  return m
})
const scores = computed(() => scoreBonds(pool.value, today.value, new Map([...icuRates.value].map(([id, v]) => [id, v.rate]))))
// Сумма вложения для расчёта «через 10 лет» и графиков; помнится в браузере
const amount = ref(Math.max(1, Number(storageGet('zoe.market.amount', '10000')) || 10000))
watch(amount, (v) => { if (v > 0) storageSet('zoe.market.amount', String(v)) })
const amountN = computed(() => (Number(amount.value) > 0 ? Number(amount.value) : 1))

// Инфляция г/г (последний известный месяц) — для колонки «Реальная»; только ОВДП в гривне
const inflation = computed(() => currentInflation(props.macro))
const strip = computed(() => {
  if (section.value !== 'nbu') return null
  const m = macroSummary(props.macro, today.value)
  return m.cpi === null ? null : { policy: m.policy, cpi: m.cpi, real: m.realPolicy, month: m.cpiMonth ? `${m.cpiMonth.slice(5, 7)}.${m.cpiMonth.slice(0, 4)}` : '' }
})
const columns = computed(() => COLUMNS.filter((c) => c.key !== 'real' || (section.value === 'nbu' && inflation.value !== null)))

const found = computed(() => filterBonds(pool.value, { q: q.value, currency: currency.value, hideMatured: hideMatured.value, today: today.value }))
const rows = computed(() => sortBonds(
  found.value.map((b) => {
    const sc = scores.value.get(b.id)
    return { ...b, score: sc?.score ?? 0, future: sc ? sc.multipleH * amountN.value : 0, real: sc && b.currency === 'UAH' ? realRate(sc.rate, inflation.value) : null }
  }),
  sort.value.key, sort.value.dir
).map((b) => {
  const sc = scores.value.get(b.id) || null
  const c = bondCells(b)
  const own = icuRates.value.get(String(b.id).toUpperCase())
  if (own) c.yield = `${own.source === 'curve' ? '≈\u00A0' : ''}${f2(own.rate)} %`
  c.score = sc ? `${Math.round(sc.score)} %` : '—'
  c.future = sc ? moneyText(b.future, b.currency) : '—'
  c.real = signedPct(b.real)
  return { id: b.id, c, sc, real: b.real, ysrc: own ? own.source : '', color: sc ? scoreColor(sc.score) : null, held: holdings.value.get(String(b.id).toUpperCase()) || 0 }
}))

// Графики: бумаги одной валюты (при «ВСЕ» — самой многочисленной среди найденных). Лучшие 5 по коэффициенту — с точками.
const CHART_LIMIT = 150
const chartCurrency = computed(() => {
  if (currency.value) return currency.value
  const n = {}
  for (const b of found.value) if (scores.value.has(b.id)) n[b.currency] = (n[b.currency] || 0) + 1
  return Object.keys(n).sort((a, b) => n[b] - n[a])[0] || ''
})
const NONE = Object.freeze([]) // стабильный пустой массив: новый [] на каждый рендер сбрасывал бы окно графика

// Разрыв между Топ 1 и Топ 5 по результату за 10 лет — по каждой валюте сразу (в процентах, не в гривнах)
const spreads = computed(() => {
  const by = {}
  for (const b of found.value) {
    const sc = scores.value.get(b.id)
    if (sc && isFinite(sc.multipleH) && sc.multipleH > 0) (by[b.currency] ||= []).push(sc)
  }
  return Object.entries(by)
    .map(([cur, arr]) => {
      const top = arr.sort((a, b) => b.score - a.score).slice(0, 5)
      const hs = top.map((x) => x.multipleH)
      return { currency: cur, n: top.length, pct: top.length > 1 ? (Math.max(...hs) / Math.min(...hs) - 1) * 100 : 0 }
    })
    .filter((s) => s.n > 1)
    .sort((a, b) => a.currency.localeCompare(b.currency))
})

const chartItems = computed(() => {
  const all = found.value
    .filter((b) => b.currency === chartCurrency.value && scores.value.has(b.id))
    .map((b) => ({ id: b.id, name: b.name || b.id, sc: scores.value.get(b.id) }))
    .sort((a, b) => b.sc.score - a.sc.score || a.id.localeCompare(b.id))
    .slice(0, CHART_LIMIT)
  return all.map((i, n) => ({ ...i, top: n < 5, place: n + 1, color: scoreColor(i.sc.score).text }))
})
// Сгоревшие (погашенные) бумаги для графика: серым пунктиром от выпуска до погашения; без даты выпуска или ставки нарисовать нечем
const showMatured = ref(false)
const MATURED_LIMIT = 100
const yearsFromToday = (iso) => (Date.parse(`${iso}T00:00:00Z`) - Date.parse(`${today.value}T00:00:00Z`)) / (365.25 * 86400000)
const chartMatured = computed(() => filterBonds(pool.value, { q: q.value, currency: chartCurrency.value || currency.value, hideMatured: false, today: today.value })
  .filter((b) => b.currency === chartCurrency.value && isMatured(b, today.value) && b.issued && rateOf(b) > 0 && b.issued < b.maturity)
  .sort((a, b) => b.maturity.localeCompare(a.maturity))
  .slice(0, MATURED_LIMIT)
  .map((b) => ({ id: b.id, rate: rateOf(b), startT: yearsFromToday(b.issued), endT: yearsFromToday(b.maturity) })))
const chartNote = computed(() => (!currency.value && currencies.value.length > 1 && chartCurrency.value ? `График: ${chartCurrency.value}. Выбери валюту, чтобы сменить.` : ''))

const YSRC_TITLE = {
  price: 'Доходность к погашению по цене в ICU (с накопленным купоном)',
  ytm: 'YTM, показанный в ICU',
  curve: 'Оценка по кривой доходности: цены этой бумаги нет, взяты соседи по сроку и точки кривой'
}
const TIER_CLASS = {
  top1: 'text-amber-300 border-amber-400/70 bg-amber-400/15',
  top10: 'text-violet-300 border-violet-400/60 bg-violet-400/10',
  top25: 'text-sky-300 border-sky-400/60 bg-sky-400/10',
  top100: 'text-slate-300 border-slate-500/60 bg-slate-500/10'
}
const f2 = (x) => (Math.round(x * 100) / 100).toFixed(2).replace(/0$/, '').replace(/\.$/, '').replace('.', ',')
const f1 = (x) => String(Math.round(x * 10) / 10).replace('.', ',')
const scoreTitle = (sc) => `Коэффициент ${Math.round(sc.score)} %. Рост капитала ≈ ${f1(sc.growth)} % годовых за ${HORIZON_YEARS} лет: ставка ${f1(sc.rate)} % на ${f1(sc.years)} г., дальше под ${f1(sc.base)} % (осторожная база ${sc.currency}: нижняя четверть ставок). Место ${sc.rank} среди ${sc.currency}. Вложение ${moneyText(amountN.value, sc.currency)}: к погашению ${moneyText(amountN.value * sc.multipleT, sc.currency)}, за ${HORIZON_YEARS} лет ${moneyText(amountN.value * sc.multipleH, sc.currency)}`
</script>

<template>
  <div class="relative space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">{{ source.title }}</h2>
      <input v-model="q" type="text" class="ui-input !bg-slate-900 !w-56" :placeholder="`Поиск: ${source.idLabel} или название`" />
      <div v-if="currencies.length > 1" class="flex items-center gap-1">
        <button
          v-for="c in ['', ...currencies]"
          :key="c || 'all'"
          type="button"
          class="text-[10px] font-bold px-2 py-1 rounded border cursor-pointer transition-colors"
          :class="c === currency ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'text-slate-500 border-transparent hover:text-slate-300'"
          @click="currency = c"
        >{{ c || 'ВСЕ' }}</button>
      </div>
      <label class="flex items-center gap-2 text-slate-400 cursor-pointer">
        <input v-model="hideMatured" type="checkbox" class="accent-emerald-500" />скрыть погашенные
      </label>
      <label class="flex items-center gap-2 text-slate-400 cursor-pointer" title="На графике: уже погашенные бумаги серым пунктиром от выпуска до погашения (нужна дата выпуска)">
        <input v-model="showMatured" type="checkbox" class="accent-emerald-500" />сгоревшие на графике
      </label>
      <template v-if="section === 'nbu'">
        <label class="flex items-center gap-2 cursor-pointer" :class="icu.isins.value.length ? 'text-slate-400' : 'text-slate-600 cursor-not-allowed'" title="Места, Топ 1 и график считаются только среди бумаг из твоего списка ICU. Снять галочку — снова весь реестр НБУ">
          <input v-model="onlyIcu" type="checkbox" class="accent-emerald-500" :disabled="!icu.isins.value.length" />только ICU<span v-if="icu.isins.value.length" class="text-slate-600">({{ icu.isins.value.length }})</span>
        </label>
        <button type="button" class="text-[10px] text-slate-500 hover:text-emerald-400 underline cursor-pointer" @click="icuOpen = !icuOpen">{{ icu.isins.value.length ? 'список ICU' : 'добавить список ICU' }}</button>
      </template>
      <label class="flex items-center gap-2 text-slate-400" title="Сумма вложения: по ней считается столбец «Через 10 л.» и графики">
        вложение
        <input v-model.number="amount" type="number" min="1" step="1000" class="ui-input !bg-slate-900 !w-28 text-right" />
      </label>
      <span class="text-slate-500">Бумаг: {{ rows.length }}<template v-if="list && rows.length !== list.length"> из {{ list.length }}</template></span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>

    <IcuIntake v-if="icuOpen && section === 'nbu'" :registry="list || []" :today="today" :icu="icu" @close="icuOpen = false" />

    <section v-if="strip" class="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-xs bg-slate-900/60 border border-slate-800 rounded px-4 py-2">
      <button type="button" class="text-slate-400 hover:text-emerald-400 cursor-pointer" title="Открыть экран «Ставка НБУ»" @click="emit('go', 'rates')">Облікова <b class="text-emerald-400 tabular-nums">{{ plainPct(strip.policy) }}</b></button>
      <button type="button" class="text-slate-400 hover:text-amber-300 cursor-pointer" :title="`Последняя публикация: ${strip.month}. Открыть экран «Инфляция»`" @click="emit('go', 'inflation')">Инфляция г/г <b class="text-amber-300 tabular-nums">{{ plainPct(strip.cpi) }}</b><span class="text-slate-600"> · {{ strip.month }}</span></button>
      <span v-if="strip.real !== null" class="text-slate-400" title="(1 + ставка) / (1 + инфляция) − 1">Реальная ставка <b class="tabular-nums" :class="strip.real >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(strip.real) }}</b></span>
    </section>

    <GrowthChart v-if="list && (chartItems.length || chartMatured.length)" :items="chartItems" :matured="showMatured ? chartMatured : NONE" :amount="amountN" :currency="chartCurrency" :today="today" :spreads="spreads" />
    <div v-if="chartNote" class="text-[11px] text-slate-500 -mt-2">{{ chartNote }}</div>

    <div v-if="startError || loadError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError || loadError }}</div>

    <div class="border border-slate-800 rounded overflow-x-auto bg-slate-900/40">
      <table class="w-full text-xs">
        <thead>
          <tr class="text-slate-500 uppercase text-[10px] tracking-wider border-b border-slate-800">
            <th
              v-for="col in columns"
              :key="col.key"
              class="px-3 py-3 font-bold cursor-pointer hover:text-slate-300 whitespace-nowrap"
              :class="col.num ? 'text-right' : 'text-left'"
              @click="sort = nextSort(sort, col.key)"
            >{{ col.key === 'id' ? source.idLabel : col.label }}<span v-if="sort.key === col.key" class="text-emerald-400 ml-1">{{ sort.dir === 'asc' ? '↑' : '↓' }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rows" :key="r.id" class="border-b border-slate-800/60 hover:bg-slate-800/40">
            <td v-for="col in columns" :key="col.key" class="px-3 py-2 whitespace-nowrap" :class="[col.num ? 'text-right tabular-nums' : 'text-left', col.key === 'id' ? 'text-slate-300' : 'text-slate-200']">
              <template v-if="col.key === 'name'">
                <span class="inline-flex items-center gap-2">
                  <span
                    class="px-2 py-0.5 rounded border text-[11px] font-bold"
                    :class="r.color ? '' : 'text-slate-500 border-slate-700 bg-slate-800/40'"
                    :style="r.color ? { color: r.color.text, borderColor: r.color.border, backgroundColor: r.color.bg } : null"
                    :title="r.sc ? scoreTitle(r.sc) : 'Нет даты погашения или ставки: коэффициент не считается'"
                  >{{ r.c.name }}</span>
                  <span v-if="r.sc && r.sc.tier" class="px-1.5 py-0.5 rounded border text-[9px] font-bold uppercase tracking-wider" :class="TIER_CLASS[r.sc.tier]" :title="`${r.sc.tierLabel} среди ${r.sc.currency}`">{{ r.sc.tierLabel }}</span>
                  <span v-if="r.held" class="px-1.5 py-0.5 rounded border text-[10px] font-bold text-emerald-300 border-emerald-500/50 bg-emerald-500/10" title="Уже куплено на брокерском счёте">{{ qtyText(r.held) }}</span>
                </span>
              </template>
              <span v-else-if="col.key === 'real'" :class="r.real === null ? 'text-slate-600' : r.real >= 0 ? 'text-emerald-400' : 'text-rose-400'" :title="r.real === null ? 'Реальная доходность считается для бумаг в гривне с известной ставкой' : `Доходность ${r.sc ? plainPct(r.sc.rate) : ''} с поправкой на инфляцию ${plainPct(inflation)} г/г`">{{ r.c.real }}</span>
              <span v-else-if="col.key === 'yield' && r.ysrc" :title="YSRC_TITLE[r.ysrc]">{{ r.c[col.key] }}</span>
              <template v-else>{{ r.c[col.key] }}</template>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td :colspan="columns.length" class="px-3 py-10 text-center text-slate-600 uppercase tracking-wider">
              <template v-if="list === undefined">Загружаю…</template>
              <template v-else-if="!list.length">Кеш пуст. Нажми «Обновить»</template>
              <template v-else>Ничего не найдено</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

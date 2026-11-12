<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import UiButton from '../components/ui/UiButton.vue'
import GrowthChart from '../components/GrowthChart.vue'
import { storageGet, storageSet } from '../utils/storage.js'
import { useSection } from '../composables/useSection.js'
import { apiGetJson, apiSend, apiJson } from '../utils/api.js'
import { toLocalDateStr } from '../utils/formatters.js'
import { SOURCES, MARKET_SECTIONS, COLUMNS, filterBonds, sortBonds, currenciesOf, bondCells, nextSort, parseIsins, parseIcuList, ytmFromPrice, holdingsOf, qtyText, restrictTo, scoreBonds, scoreColor, HORIZON_YEARS, moneyText, isMatured, rateOf } from '../utils/marketTable.js'

// dataVersion растёт при любой записи в кеш (в том числе после опроса рынка) — по нему перечитываем список.
// running — идёт ли сейчас опрос рынка (из /sync/status → market.running).
const props = defineProps({
  accounts: { type: Array, default: () => [] }, // счета: из брокерских (ICU) берутся уже купленные бумаги
  dataVersion: { default: null },
  running: { type: Boolean, default: false }
})

const section = useSection('economic_market_section', MARKET_SECTIONS, 'nbu')
const sections = SOURCES.map(({ key, label, icon }) => ({ key, label, icon }))
const source = computed(() => SOURCES.find((s) => s.key === section.value) || SOURCES[0])

const lists = ref({}) // source → список; у каждого источника свой, чтобы возврат на вкладку не мигал пустотой
const loadError = ref('')
const startError = ref('')

const load = async (src) => {
  try {
    lists.value = { ...lists.value, [src]: (await apiGetJson(`/market/bonds?source=${src}`)) || [] }
    loadError.value = ''
  } catch (e) {
    loadError.value = String(e.message || e)
  }
}
onMounted(() => { load(section.value); loadIcu() })
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
// «Только ICU»: свой список ISIN (вставляется из брокера, помнится в браузере); места и «Топ 1» считаются среди этих бумаг.
const icuPrices = ref({}) // ISIN → цена за бумагу в брокере (с накопленным купоном): по ней считается доходность
const icuIds = ref([]) // хранится на сервере (core.db, список «icu»): переживает смену браузера и «стереть кеш»
const onlyIcu = ref(storageGet('zoe.market.onlyIcu', '0') === '1')
const icuOpen = ref(false)
const icuText = ref('')
const icuSet = computed(() => new Set(icuIds.value))
const icuActive = computed(() => section.value === 'nbu' && onlyIcu.value && icuSet.value.size > 0)
watch(onlyIcu, (v) => storageSet('zoe.market.onlyIcu', v ? '1' : '0'))
// Уже куплено (штук): позиции брокерских счетов; обновляется вместе с данными
const holdings = ref(new Map())
const loadHoldings = async () => {
  const icu = (props.accounts || []).filter((a) => a.bank_key === 'icu')
  try {
    holdings.value = holdingsOf(await Promise.all(icu.map((a) => apiGetJson(`/icu/ledger?account_id=${encodeURIComponent(a.id)}`))))
  } catch (e) {
    holdings.value = new Map()
  }
}
watch(() => [props.accounts, props.dataVersion], loadHoldings, { immediate: true })

const icuError = ref('')
const putIcu = async (ids, prices = {}) => {
  const r = await apiJson('PUT', '/market/lists?name=icu', { isins: ids, prices })
  icuPrices.value = r.prices || {}
  return r.isins || []
}
const loadIcu = async () => {
  try {
    const got = await apiGetJson('/market/lists?name=icu')
    let ids = got.isins || []
    icuPrices.value = got.prices || {}
    const old = parseIsins(storageGet('zoe.market.icu', '')) // список из прошлой версии (только браузер): переносим на сервер один раз
    if (!ids.length && old.length) { ids = await putIcu(old); storageSet('zoe.market.icu', '') }
    icuIds.value = ids
    if (!ids.length) onlyIcu.value = false
  } catch (e) {
    icuError.value = String(e.message || e)
  }
}
const saveIcu = async () => {
  try {
    const parsed = parseIcuList(icuText.value)
    icuIds.value = await putIcu(parsed.isins, parsed.prices)
    icuError.value = ''
    if (!icuIds.value.length) onlyIcu.value = false
    icuOpen.value = false
  } catch (e) {
    icuError.value = String(e.message || e)
  }
}
const openIcu = () => { icuText.value = icuIds.value.map((id) => (icuPrices.value[id] ? `${id} ${icuPrices.value[id]}` : id)).join('\n'); icuOpen.value = !icuOpen.value }
const pool = computed(() => (icuActive.value ? restrictTo(list.value, icuSet.value) : list.value))
// Доходность по цене брокера (только в режиме «только ICU»: у бумаг без цены остаётся аукционная или купонная)
const icuRates = computed(() => {
  const m = new Map()
  if (!icuActive.value) return m
  for (const b of pool.value || []) {
    const price = icuPrices.value[String(b.id).toUpperCase()]
    const y = price ? ytmFromPrice(b, price, today.value) : null
    if (y > 0) m.set(String(b.id).toUpperCase(), y)
  }
  return m
})
const scores = computed(() => scoreBonds(pool.value, today.value, icuRates.value))
// Сумма вложения для расчёта «через 10 лет» и графиков; помнится в браузере
const amount = ref(Math.max(1, Number(storageGet('zoe.market.amount', '10000')) || 10000))
watch(amount, (v) => { if (v > 0) storageSet('zoe.market.amount', String(v)) })
const amountN = computed(() => (Number(amount.value) > 0 ? Number(amount.value) : 1))

const found = computed(() => filterBonds(pool.value, { q: q.value, currency: currency.value, hideMatured: hideMatured.value, today: today.value }))
const rows = computed(() => sortBonds(
  found.value.map((b) => {
    const sc = scores.value.get(b.id)
    return { ...b, score: sc?.score ?? 0, future: sc ? sc.multipleH * amountN.value : 0 }
  }),
  sort.value.key, sort.value.dir
).map((b) => {
  const sc = scores.value.get(b.id) || null
  const c = bondCells(b)
  const own = icuRates.value.get(String(b.id).toUpperCase())
  if (own) c.yield = `${f2(own)} %`
  c.score = sc ? `${Math.round(sc.score)} %` : '—'
  c.future = sc ? moneyText(b.future, b.currency) : '—'
  return { id: b.id, c, sc, color: sc ? scoreColor(sc.score) : null, held: holdings.value.get(String(b.id).toUpperCase()) || 0 }
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
    <SectionRail v-model="section" :sections="sections" label="Разделы рынка" />

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
        <label class="flex items-center gap-2 cursor-pointer" :class="icuIds.length ? 'text-slate-400' : 'text-slate-600 cursor-not-allowed'" title="Места, Топ 1 и график считаются только среди бумаг из твоего списка ICU. Снять галочку — снова весь реестр НБУ">
          <input v-model="onlyIcu" type="checkbox" class="accent-emerald-500" :disabled="!icuIds.length" />только ICU<span v-if="icuIds.length" class="text-slate-600">({{ icuIds.length }})</span>
        </label>
        <button type="button" class="text-[10px] text-slate-500 hover:text-emerald-400 underline cursor-pointer" @click="openIcu">{{ icuIds.length ? 'список ICU' : 'добавить список ICU' }}</button>
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

    <section v-if="icuOpen && section === 'nbu'" class="bg-slate-900/60 border border-slate-800 rounded p-3 space-y-2 text-xs">
      <div class="text-slate-400">Вставь текст из ICU (экран «Купити» → ОВДП): ISIN и цена за бумагу (например «1 117,72») вытащатся сами, остальное игнорируется. По цене считается доходность; без цены берётся ставка из реестра. Список хранится на сервере; сюда можно дописывать новые ISIN, а то, что ушло из ICU, удалить.</div>
      <textarea v-model="icuText" rows="4" class="ui-input !bg-slate-900 w-full font-mono" placeholder="UA4000239040&#10;UA4000239008&#10;…"></textarea>
      <div class="flex items-center gap-3">
        <UiButton variant="secondary" size="sm" @click="saveIcu">СОХРАНИТЬ</UiButton>
        <span class="text-slate-500">найдено ISIN: {{ parseIcuList(icuText).isins.length }}, с ценой: {{ Object.keys(parseIcuList(icuText).prices).length }}</span>
        <span v-if="icuError" class="text-rose-400">{{ icuError }}</span>
        <button type="button" class="text-slate-500 hover:text-slate-300 underline cursor-pointer" @click="icuOpen = false">закрыть</button>
      </div>
    </section>

    <GrowthChart v-if="list && (chartItems.length || chartMatured.length)" :items="chartItems" :matured="showMatured ? chartMatured : NONE" :amount="amountN" :currency="chartCurrency" :today="today" :spreads="spreads" />
    <div v-if="chartNote" class="text-[11px] text-slate-500 -mt-2">{{ chartNote }}</div>

    <div v-if="startError || loadError" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">{{ startError || loadError }}</div>

    <div class="border border-slate-800 rounded overflow-x-auto bg-slate-900/40">
      <table class="w-full text-xs">
        <thead>
          <tr class="text-slate-500 uppercase text-[10px] tracking-wider border-b border-slate-800">
            <th
              v-for="col in COLUMNS"
              :key="col.key"
              class="px-3 py-3 font-bold cursor-pointer hover:text-slate-300 whitespace-nowrap"
              :class="col.num ? 'text-right' : 'text-left'"
              @click="sort = nextSort(sort, col.key)"
            >{{ col.key === 'id' ? source.idLabel : col.label }}<span v-if="sort.key === col.key" class="text-emerald-400 ml-1">{{ sort.dir === 'asc' ? '↑' : '↓' }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rows" :key="r.id" class="border-b border-slate-800/60 hover:bg-slate-800/40">
            <td v-for="col in COLUMNS" :key="col.key" class="px-3 py-2 whitespace-nowrap" :class="[col.num ? 'text-right tabular-nums' : 'text-left', col.key === 'id' ? 'text-slate-300' : 'text-slate-200']">
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
              <template v-else>{{ r.c[col.key] }}</template>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td :colspan="COLUMNS.length" class="px-3 py-10 text-center text-slate-600 uppercase tracking-wider">
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

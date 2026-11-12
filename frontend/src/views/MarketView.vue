<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useSection } from '../composables/useSection.js'
import { apiGetJson, apiSend } from '../utils/api.js'
import { toLocalDateStr } from '../utils/formatters.js'
import { SOURCES, MARKET_SECTIONS, COLUMNS, filterBonds, sortBonds, currenciesOf, bondCells, nextSort, scoreBonds, scoreColor } from '../utils/marketTable.js'

// dataVersion растёт при любой записи в кеш (в том числе после опроса рынка) — по нему перечитываем список.
// running — идёт ли сейчас опрос рынка (из /sync/status → market.running).
const props = defineProps({
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
onMounted(() => load(section.value))
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
const scores = computed(() => scoreBonds(list.value, today.value))
const rows = computed(() => sortBonds(
  filterBonds(list.value, { q: q.value, currency: currency.value, hideMatured: hideMatured.value, today: today.value })
    .map((b) => ({ ...b, score: scores.value.get(b.id)?.score ?? 0 })),
  sort.value.key, sort.value.dir
).map((b) => {
  const sc = scores.value.get(b.id) || null
  const c = bondCells(b)
  c.score = sc ? `${Math.round(sc.score)} %` : '—'
  return { id: b.id, c, sc, color: sc ? scoreColor(sc.score) : null }
}))

const TIER_CLASS = {
  top1: 'text-amber-300 border-amber-400/70 bg-amber-400/15',
  top10: 'text-violet-300 border-violet-400/60 bg-violet-400/10',
  top25: 'text-sky-300 border-sky-400/60 bg-sky-400/10',
  top100: 'text-slate-300 border-slate-500/60 bg-slate-500/10'
}
const scoreTitle = (sc) => `Коэффициент ${Math.round(sc.score)} %: срок ${Math.round(sc.term)} % · ставка ${Math.round(sc.rate)} %. Место ${sc.rank} среди ${sc.currency}`

const refresh = async () => {
  startError.value = ''
  try {
    await apiSend('POST', `/market/sync?source=${section.value}`)
  } catch (e) {
    startError.value = String(e.message || e)
  }
}
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
      <span class="text-slate-500">Бумаг: {{ rows.length }}<template v-if="list && rows.length !== list.length"> из {{ list.length }}</template></span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>

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

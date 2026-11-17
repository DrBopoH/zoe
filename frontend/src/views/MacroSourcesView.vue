<script setup>
import { intText } from '../utils/numfmt.js'
import { computed, ref } from 'vue'
import MarketHeader from '../components/MarketHeader.vue'
import { apiGetJson, apiSend, q } from '../utils/api.js'
import { pushRequestError } from '../composables/useToasts.js'
import { dateOf } from '../utils/macro.js'
import { feedStatus, STATUS_TEXT, agoText, lagText, seriesRows, filterSeries, seriesCsv, healthSummary } from '../utils/sources.js'
import UiCard from '../components/ui/UiCard.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'

// Экран «Источники»: здоровье каждого источника (что обещает, что в кеше, когда обновлялся, какие ряды устарели, последняя ошибка)
// и каталог всех рядов с поиском и выгрузкой в CSV. Данные считает сервер; кнопка в шапке запускает опрос всех источников, кнопка в строке — одного.
const props = defineProps({
  health: { type: Object, default: () => ({}) }, // ответ GET /api/macro/sources
  running: { type: Boolean, default: false }
})

const feeds = computed(() => props.health.feeds || [])
const rows = computed(() => seriesRows(props.health))
const summary = computed(() => healthSummary(feeds.value))
const query = ref('')
const feed = ref('')
const staleOnly = ref(false)
const LIMIT = 200
const found = computed(() => filterSeries(rows.value, { query: query.value, feed: feed.value, staleOnly: staleOnly.value }))
const shown = computed(() => found.value.slice(0, LIMIT))

const refreshAll = () => start('')
const start = async (source) => {
  try {
    await apiSend('POST', `/market/sync${source ? `?source=${q(source)}` : ''}`)
  } catch (e) {
    pushRequestError('Не удалось обновить', e)
  }
}
const download = async (id) => {
  try {
    const r = await apiGetJson(`/macro?series=${q(id)}`)
    const blob = new Blob([seriesCsv(r?.series?.[id])], { type: 'text/csv;charset=utf-8' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `${id}.csv`
    a.click()
    setTimeout(() => URL.revokeObjectURL(a.href), 1000)
  } catch (e) {
    pushRequestError('Не удалось скачать ряд', e)
  }
}

const TONE = { error: 'text-rose-400', empty: 'text-slate-500', stale: 'text-amber-300', partial: 'text-amber-300', ok: 'text-emerald-400' }
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Источники" :running="running" @refresh="refreshAll">
      <span class="text-slate-500">
        свежих {{ summary.ok }} из {{ summary.total }}<template v-if="summary.stale"> · с устаревшими рядами {{ summary.stale }}</template><template v-if="summary.error"> · с ошибкой {{ summary.error }}</template><template v-if="summary.empty"> · пустых {{ summary.empty }}</template>
      </span>
    </MarketHeader>

    <UiCard class="text-xs overflow-x-auto">
      <table v-if="feeds.length" class="w-full tabular-nums">
        <thead>
          <tr class="text-slate-500">
            <th class="px-3 py-2 text-left font-normal">Источник</th>
            <th class="px-3 py-2 text-left font-normal">Состояние</th>
            <th class="px-3 py-2 text-right font-normal">Рядов</th>
            <th class="px-3 py-2 text-right font-normal">Точек</th>
            <th class="px-3 py-2 text-right font-normal">Последняя точка</th>
            <th class="px-3 py-2 text-right font-normal">Обновлён</th>
            <th class="px-3 py-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in feeds" :key="f.id" class="border-t border-slate-800/60 align-top">
            <td class="px-3 py-1.5 text-slate-300">{{ f.title }}<div v-if="f.error" class="mt-0.5 text-rose-400/80">{{ f.error }}</div></td>
            <td class="px-3 py-1.5 font-bold" :class="TONE[feedStatus(f)]">{{ STATUS_TEXT[feedStatus(f)] }}<span v-if="f.stale" class="ml-1 font-normal text-slate-500">({{ f.stale }})</span></td>
            <td class="px-3 py-1.5 text-right text-slate-300">{{ f.cached }} / {{ f.series }}</td>
            <td class="px-3 py-1.5 text-right text-slate-400">{{ intText(f.points) }}</td>
            <td class="px-3 py-1.5 text-right text-slate-300">{{ f.last_date ? dateOf(f.last_date) : '—' }}</td>
            <td class="px-3 py-1.5 text-right text-slate-400">{{ agoText(f.updated_at) }}</td>
            <td class="px-3 py-1.5 text-right"><button type="button" class="ui-btn ui-btn--sm ui-btn--outline" :disabled="running" @click="start(f.id)">Обновить</button></td>
          </tr>
        </tbody>
      </table>
      <UiEmpty v-else>Нет данных. Нажми «Обновить» — источники подтянутся, и тут появится их состояние.</UiEmpty>
    </UiCard>

    <UiCard class="text-xs">
      <div class="flex flex-wrap items-center gap-3 p-3">
        <b class="text-slate-200">Каталог рядов</b>
        <input v-model="query" type="search" placeholder="Поиск: id, название, источник" class="ui-input !bg-slate-900 !w-64" aria-label="Поиск по рядам" />
        <select v-model="feed" class="ui-input !bg-slate-900 !w-56" aria-label="Источник">
          <option value="">Все источники</option>
          <option v-for="f in feeds" :key="f.id" :value="f.id">{{ f.title }}</option>
        </select>
        <label class="flex items-center gap-1.5 text-slate-400"><input v-model="staleOnly" type="checkbox" /> только устаревшие</label>
        <span class="ml-auto text-slate-500">найдено {{ found.length }}<template v-if="found.length > LIMIT">, показано {{ LIMIT }}: уточни запрос</template></span>
      </div>
      <div class="overflow-x-auto">
        <table v-if="shown.length" class="w-full tabular-nums">
          <thead>
            <tr class="text-slate-500">
              <th class="px-3 py-1 text-left font-normal">Ряд</th>
              <th class="px-3 py-1 text-left font-normal">Источник</th>
              <th class="px-3 py-1 text-left font-normal">Частота</th>
              <th class="px-3 py-1 text-right font-normal">Точек</th>
              <th class="px-3 py-1 text-right font-normal">С</th>
              <th class="px-3 py-1 text-right font-normal">По</th>
              <th class="px-3 py-1 text-right font-normal">Отставание</th>
              <th class="px-3 py-1"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in shown" :key="r.id" class="border-t border-slate-800/60">
              <td class="px-3 py-1.5"><span class="text-slate-200">{{ r.label || r.id }}</span><div v-if="r.label" class="text-slate-500">{{ r.id }}</div></td>
              <td class="px-3 py-1.5 text-slate-400">{{ r.feedTitle || '—' }}</td>
              <td class="px-3 py-1.5 text-slate-400">{{ r.freq }}</td>
              <td class="px-3 py-1.5 text-right text-slate-400">{{ intText(r.points) }}</td>
              <td class="px-3 py-1.5 text-right text-slate-400">{{ dateOf(r.from) }}</td>
              <td class="px-3 py-1.5 text-right text-slate-300">{{ dateOf(r.to) }}</td>
              <td class="px-3 py-1.5 text-right" :class="r.stale ? 'text-amber-300' : 'text-slate-400'">{{ lagText(r.lag_days) }}</td>
              <td class="px-3 py-1.5 text-right"><button type="button" class="ui-btn ui-btn--sm ui-btn--outline" @click="download(r.id)">CSV</button></td>
            </tr>
          </tbody>
        </table>
        <UiEmpty v-else>{{ rows.length ? 'Ничего не найдено.' : 'Кеш пуст: нажми «Обновить».' }}</UiEmpty>
      </div>
    </UiCard>
  </div>
</template>

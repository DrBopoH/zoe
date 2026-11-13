<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiIcon from './ui/UiIcon.vue'
import CoverBar from './CoverBar.vue'
import ImportCsvModal from './ImportCsvModal.vue'
import SyncButton from './SyncButton.vue'
import { apiGetJson, apiSend, q } from '../utils/api.js'
import { autoSave, setAutoSave } from '../composables/useSync.js'
import { formatRuDate, formatMaskedIBAN, plural, fmtN, formatPeriod } from '../utils/formatters.js'

// Окно «ВЫПИСКИ»: всё, что известно об источниках данных (файлы выписок, снимки API) и об их присутствии в кеш-базе.
// У каждого счёта бит: 1 — данные в кеше, 0 — нет. «Сбросить» зануляет биты, «Загрузить» взводит их из файлов-источников.
// Если ничего не выбрано — действие для всех счетов.
const props = defineProps({ isOpen: { type: Boolean, default: false } })
const emit = defineEmits(['close', 'refresh'])

const accounts = ref([])
const info = ref(null) // { dir, migrated, failed }
const picked = ref(new Set())
const opened = ref(new Set()) // счета с раскрытым списком файлов
const busy = ref('') // reset | load | del
const msg = ref(null) // { ok, text }
const loading = ref(false)
const addOpen = ref(false)
const updateFor = ref(null) // счёт, который обновляют (строка → «Обновить»)

const load = async () => {
  loading.value = true
  try {
    const d = await apiGetJson('/sources')
    accounts.value = d.accounts || []
    info.value = d
    const ids = new Set(accounts.value.map(a => a.id))
    picked.value = new Set([...picked.value].filter(id => ids.has(id)))
  } catch (e) {
    msg.value = { ok: false, text: e.message }
  } finally {
    loading.value = false
  }
}

watch(() => props.isOpen, (v) => {
  if (v) { msg.value = null; load() } else { arm.value = ''; }
})

const toggle = (id) => {
  const s = new Set(picked.value)
  s.has(id) ? s.delete(id) : s.add(id)
  picked.value = s
}
const toggleOpen = (id) => {
  const s = new Set(opened.value)
  s.has(id) ? s.delete(id) : s.add(id)
  opened.value = s
}

const scopeIds = computed(() => [...picked.value])
const scopeText = computed(() => (picked.value.size ? `${picked.value.size}` : 'все'))
const scopeSuffix = computed(() => (picked.value.size ? ` (${picked.value.size})` : ''))

// двойное нажатие для разрушающих действий
const arm = ref('')
let armTimer = null
const armed = (key) => arm.value === key
const confirm = (key, run) => {
  if (arm.value === key) { clearTimeout(armTimer); arm.value = ''; run(); return }
  arm.value = key
  clearTimeout(armTimer)
  armTimer = setTimeout(() => { arm.value = '' }, 4000)
}
onBeforeUnmount(() => clearTimeout(armTimer))

const run = async (kind, fn) => {
  busy.value = kind
  msg.value = null
  try {
    await fn()
  } catch (e) {
    msg.value = { ok: false, text: e.message }
  } finally {
    busy.value = ''
    await load()
    emit('refresh')
  }
}

const doReset = () => run('reset', async () => {
  await apiSend('POST', '/sources/reset', { accounts: scopeIds.value })
  msg.value = { ok: true, text: `Кэш сброшен: ${scopeText.value === 'все' ? 'все счета' : scopeText.value + ' ' + plural(picked.value.size, 'счёт', 'счёта', 'счетов')}. Файлы-источники на месте.` }
})

const doLoad = () => run('load', async () => {
  const d = await (await apiSend('POST', '/sources/load', { accounts: scopeIds.value })).json()
  const bad = d.failed?.length || 0
  if (bad) console.error('Sources load failures:', d.failed)
  msg.value = {
    ok: bad === 0,
    text: d.total === 0 ? 'Нет файлов-источников для загрузки' : `Загружено ${d.loaded} из ${d.total}` + (bad ? ` · ошибок: ${bad} — ${d.failed.map(f => `${f.file}: ${f.error}`).join('; ')}` : '')
  }
})

const doDelete = (acc, f) => run('del', async () => {
  const d = await (await apiSend('DELETE', `/sources/file?id=${q(f.id)}`)).json()
  msg.value = { ok: true, text: d.refetch ? 'Снимок API удалён: следующая синхронизация стянет этот счёт с API заново.' : 'Файл-источник удалён. Данные в кэше остались до сброса.' }
})

const ORIGIN = { csv: 'CSV', pdf: 'PDF', report: 'ОТЧЁТ', binance: 'ВЫПИСКА', api: 'API' }
const originLabel = (f) => (f.kind === 'binance-series' ? 'РЯД API' : f.kind === 'binance-p2p' ? 'P2P' : ORIGIN[f.origin] || f.origin)
const originTone = (f) => (f.origin === 'api' ? 'text-sky-300 border-sky-500/40' : 'text-slate-300 border-slate-600')

const period = (f) => formatPeriod(f.from, f.to)
const label = (a) => (/^[A-Z]{2}\d{2}/.test(a.label || '') && a.label.length > 12 ? formatMaskedIBAN(a.label) : a.label)
const hasApiFile = (a) => a.files.some(f => f.origin === 'api')

</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-4xl" @close="emit('close')">
    <!-- Шапка -->
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-sm font-bold text-white uppercase tracking-wider">Выписки</span>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <UiButton variant="primary" @click="addOpen = true">Добавить</UiButton>
        <UiIconButton icon="close" title="Закрыть" @click="emit('close')" />
      </div>
    </div>

    <div v-if="msg" class="ui-banner break-words" :class="msg.ok ? 'ui-banner--success' : 'ui-banner--error'">{{ msg.text }}</div>
    <div v-if="info?.migrated" class="ui-banner ui-banner--success">Старых файлов переведено в новый формат: {{ info.migrated }}</div>
    <div v-for="(f, i) in info?.failed || []" :key="'bad' + i" class="ui-banner ui-banner--error break-words">Не разобран: {{ f.file }} — {{ f.error }}</div>

    <!-- Счета -->
    <div class="space-y-2 max-h-[60vh] overflow-y-auto pr-1">
      <p v-if="!accounts.length && !loading" class="text-center text-slate-500 py-8">Источников пока нет. Нажми «Добавить» и выбери выписку.</p>

      <div v-for="a in accounts" :key="a.id" class="rounded border p-3 space-y-2 transition-colors" :class="picked.has(a.id) ? 'border-emerald-500/50 bg-emerald-950/10' : 'border-slate-800 bg-slate-950/30'">
        <div class="flex items-start gap-3">
          <input type="checkbox" class="accent-emerald-500 mt-1" :checked="picked.has(a.id)" @change="toggle(a.id)" />
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-bold text-slate-100 truncate">{{ a.name || a.id }}</span>
              <span class="text-[9px] font-bold px-1.5 py-0.5 rounded border border-slate-600 text-slate-300 uppercase">{{ a.type }}</span>
              <span v-if="label(a)" class="text-[10px] text-slate-500">{{ label(a) }}</span>
            </div>
            <div class="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[10px]">
              <span :class="a.loaded ? 'text-emerald-400' : 'text-slate-500'">{{ a.loaded ? `● в кэше · ${fmtN(a.cached)}` : '○ нет в кэше' }}</span>
              <span class="text-slate-500">{{ a.files.length ? `${a.files.length} ${plural(a.files.length, 'источник', 'источника', 'источников')} · ${fmtN(a.records)} ${plural(a.records, 'запись', 'записи', 'записей')}` : 'нет файла-источника' }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <button v-if="a.files.length" type="button" class="text-[10px] text-slate-400 hover:text-slate-200" @click="toggleOpen(a.id)">
              {{ opened.has(a.id) ? '▾ Скрыть' : '▸ Источники' }}
            </button>
            <UiButton variant="primary" size="sm" :disabled="!!busy" @click="updateFor = { id: a.id, name: a.name }">Обновить</UiButton>
          </div>
        </div>

        <CoverBar :cells="a.cells" :from="a.axis_from" :to="a.axis_to" />

        <!-- Файлы-источники -->
        <div v-if="opened.has(a.id)" class="space-y-1 pt-1 border-t border-slate-800">
          <div v-for="f in a.files" :key="f.id" class="flex items-center gap-2 text-[10px] py-1">
            <span class="font-bold px-1.5 py-0.5 rounded border shrink-0" :class="originTone(f)">{{ originLabel(f) }}</span>
            <span class="text-slate-200 truncate min-w-0 flex-1" :title="f.name">{{ f.name }}</span>
            <span class="text-slate-400 whitespace-nowrap w-60 text-right">{{ period(f) }}</span>
            <span class="text-slate-500 whitespace-nowrap w-20 text-right">{{ fmtN(f.count) }}</span>
            <span class="text-slate-600 whitespace-nowrap hidden sm:inline" :title="'Создан ' + formatRuDate(f.created)">{{ formatRuDate(f.created) }}</span>
            <button
              type="button"
              class="shrink-0 px-1.5 py-0.5 rounded border transition-colors"
              :class="armed('del' + f.id) ? 'border-rose-400 text-rose-300 bg-rose-950/40' : 'border-transparent text-slate-500 hover:text-rose-400'"
              :disabled="!!busy"
              :title="f.origin === 'api' ? 'Снести снимок API: следующая синхронизация стянет всё заново' : 'Удалить файл-источник'"
              @click="confirm('del' + f.id, () => doDelete(a, f))"
            >
              <span v-if="armed('del' + f.id)">Точно?</span>
              <UiIcon v-else name="trash" size="sm" />
            </button>
          </div>
          <p v-if="hasApiFile(a)" class="text-[9px] text-slate-600">API-снимок хранит и снятые периоды: пока он на месте, они заново не качаются.</p>
        </div>
      </div>
    </div>

    <!-- Действия: без выбора — на все счета, с выбором — на отмеченные (число в скобках) -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p v-if="info?.dir" class="text-[9px] text-slate-600 break-all min-w-0">Папка источников: {{ info.dir }}</p>
      <span v-else></span>
      <div class="flex items-start gap-2 shrink-0 ml-auto">
        <UiButton variant="danger" size="sm" :disabled="!!busy || !accounts.length" @click="confirm('reset', doReset)">
          {{ armed('reset') ? 'Точно? Сбросить' : `Сбросить${scopeSuffix}` }}
        </UiButton>
        <UiButton variant="primary" size="sm" :disabled="!!busy || !accounts.length" @click="doLoad">
          {{ busy === 'load' ? 'Загрузка…' : `Загрузить${scopeSuffix}` }}
        </UiButton>
        <!-- «Разведать» вместо «Сохранить с API»: обход банков; под кнопкой галочка — дописать снятое в источники, когда он закончится.
             Пустое место отделяет «Разведать» от кнопок работы с кэшем слева -->
        <div class="ml-8 flex flex-col items-end gap-1.5">
          <SyncButton size="sm" />
          <label class="flex items-center gap-1.5 text-[10px] text-slate-400 cursor-pointer select-none" title="После обхода снятое с API записывается в файлы-источники: после сброса кэш восстановится с диска, а снятые периоды заново не качаются">
            дописывать в источники
            <input type="checkbox" class="accent-emerald-500" :checked="autoSave" @change="setAutoSave($event.target.checked)" />
          </label>
        </div>
      </div>
    </div>

    <ImportCsvModal :is-open="addOpen" mode="add" @close="addOpen = false" @success="load(); emit('refresh')" />
    <ImportCsvModal :is-open="!!updateFor" mode="update" :account="updateFor" @close="updateFor = null" @success="load(); emit('refresh')" />
  </UiModal>
</template>

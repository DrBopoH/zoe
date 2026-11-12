<script setup>
import { ref, computed, watch } from 'vue'
import { apiUpload } from '../utils/api.js'
import { formatRuDate, fmtN, formatPeriod as period } from '../utils/formatters.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'

// Окно выбора файла выписки (клик или перетаскивание). Два режима:
//   add    — «Добавить»: новый счёт. Если такой счёт уже есть, предлагается обновить его этим файлом.
//   update — «Обновить» в строке счёта: файл обязан относиться именно к этому счёту, иначе отказ.
// Перед записью файл проверяется на бэкенде (/sources/inspect): что за счёт, какой алиас, как ложится на уже имеющееся.
const props = defineProps({
  isOpen: { type: Boolean, default: false },
  mode: { type: String, default: 'add' }, // add | update
  account: { type: Object, default: null } // { id, name } — для update
})

const emit = defineEmits(['close', 'success'])

const file = ref(null)
const uploading = ref(false)
const checking = ref(false)
const statusMsg = ref(null)
const fileInputRef = ref(null)
const info = ref(null) // ответ /sources/inspect

// решения пользователя
const updateExisting = ref(false)
const alias = ref('keep') // keep | replace
const force = ref(false)

const reset = () => {
  file.value = null
  statusMsg.value = null
  info.value = null
  updateExisting.value = false
  alias.value = 'keep'
  force.value = false
}
watch(() => props.isOpen, (v) => { if (v) reset() })

const has = (code) => info.value?.problems?.find(p => p.code === code)
const blocked = computed(() => !!(has('duplicate') || has('other_account')))
const needsUpdateOk = computed(() => !!has('exists'))
const canSubmit = computed(() =>
  !!file.value && !!info.value && !checking.value && !uploading.value && !blocked.value &&
  (!needsUpdateOk.value || updateExisting.value) && (!has('no_match') || force.value)
)

const inspect = async () => {
  checking.value = true
  info.value = null
  statusMsg.value = null
  try {
    const fd = new FormData()
    fd.append('file', file.value)
    if (props.mode === 'update' && props.account) fd.append('account', props.account.id)
    info.value = await (await apiUpload('/sources/inspect', fd)).json()
  } catch (err) {
    statusMsg.value = { type: 'error', text: err.message }
  } finally {
    checking.value = false
  }
}

const take = (f) => {
  const name = f.name.toLowerCase()
  if (!(name.endsWith('.csv') || name.endsWith('.pdf'))) {
    statusMsg.value = { type: 'error', text: 'Выбери файл в формате .csv или .pdf' }
    return
  }
  file.value = f
  updateExisting.value = false
  alias.value = 'keep'
  force.value = false
  inspect()
}

const handleDrop = (e) => {
  e.preventDefault()
  if (e.dataTransfer.files && e.dataTransfer.files[0]) take(e.dataTransfer.files[0])
}
const handleFileSelect = (e) => {
  if (e.target.files && e.target.files[0]) take(e.target.files[0])
}

const submitImport = async () => {
  if (!canSubmit.value) return
  uploading.value = true
  statusMsg.value = null
  const fd = new FormData()
  fd.append('file', file.value)
  const updating = props.mode === 'update' || updateExisting.value
  fd.append('mode', updating ? 'update' : 'add')
  fd.append('account', props.mode === 'update' && props.account ? props.account.id : info.value.account_id)
  if (updating) fd.append('update', '1')
  if (info.value.exists) fd.append('alias', alias.value)
  if (force.value) fd.append('force', '1')

  try {
    const data = await (await apiUpload('/sync/import-csv', fd)).json()
    statusMsg.value = { type: 'success', text: `Загружено операций: ${data.imported_count || 0}` }
    setTimeout(() => { emit('success'); emit('close') }, 900)
  } catch (err) {
    statusMsg.value = { type: 'error', text: err.message }
  } finally {
    uploading.value = false
  }
}

const title = computed(() => (props.mode === 'update' ? 'Обновить счёт' : 'Добавить счёт'))
const submitLabel = computed(() => {
  if (uploading.value) return 'Загрузка...'
  return props.mode === 'update' || updateExisting.value ? 'Обновить' : 'Добавить'
})
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-lg" @close="emit('close')">
    <!-- Шапка -->
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0">
        <span class="text-sm font-bold text-white uppercase tracking-wider">{{ title }}</span>
        <p v-if="mode === 'update' && account" class="text-[10px] text-slate-400 truncate">{{ account.name }}</p>
      </div>
      <UiIconButton icon="close" title="Закрыть" @click="emit('close')" />
    </div>

    <p class="text-[10px] text-slate-500 leading-relaxed">
      Алиас владельца берётся из имени файла (например «Петро Іваненко.csv»).
      {{ mode === 'update' ? 'Файл должен относиться к этому же счёту.' : 'Так создаётся новый счёт; выписку уже имеющегося счёта можно добавить кнопкой «Обновить» в его строке.' }}
    </p>

    <!-- Зона Drag and Drop -->
    <div
      @dragover.prevent
      @drop="handleDrop"
      @click="fileInputRef?.click()"
      class="border-2 border-dashed rounded-lg p-5 text-center cursor-pointer transition-all bg-slate-950/50"
      :class="file ? 'border-emerald-500/60 bg-emerald-950/10' : 'border-slate-800 hover:border-slate-600'"
    >
      <input ref="fileInputRef" type="file" accept=".csv,.pdf" class="hidden" @change="handleFileSelect" />

      <div v-if="file" class="space-y-1">
        <p class="font-bold text-emerald-400 truncate">{{ file.name }}</p>
        <p class="text-[10px] text-slate-500">{{ (file.size / 1024).toFixed(1) }} KB · нажми, чтобы выбрать другой</p>
      </div>
      <div v-else class="space-y-2">
        <p class="text-slate-300 font-bold">Перетащи `.csv` или `.pdf` файл сюда</p>
        <p class="text-[10px] text-slate-500">или кликни для выбора</p>
      </div>
    </div>

    <p v-if="checking" class="text-[10px] text-slate-500">Проверяю файл…</p>

    <!-- Что это за файл -->
    <div v-if="info" class="rounded border border-slate-800 bg-slate-950/40 p-3 space-y-1.5 text-[11px]">
      <div class="flex justify-between gap-3"><span class="text-slate-500">Счёт</span><span class="text-slate-200 text-right break-all">{{ info.label || info.account_id }}</span></div>
      <div class="flex justify-between gap-3"><span class="text-slate-500">Алиас</span><span class="text-slate-200 text-right break-all">{{ info.alias }}</span></div>
      <div class="flex justify-between gap-3"><span class="text-slate-500">Период файла</span><span class="text-slate-200">{{ period(info.from, info.to) }}</span></div>
      <div class="flex justify-between gap-3"><span class="text-slate-500">Записей</span><span class="text-slate-200">{{ fmtN(info.count) }}</span></div>
      <template v-if="info.exists">
        <div class="border-t border-slate-800 pt-1.5 flex justify-between gap-3"><span class="text-slate-500">Уже есть</span><span class="text-slate-300">{{ period(info.existing_from, info.existing_to) }}</span></div>
        <div v-if="info.overlap" class="flex justify-between gap-3">
          <span class="text-slate-500">В общем периоде</span>
          <span class="text-slate-300">совпало {{ fmtN(info.overlap.known) }} · новых {{ fmtN(info.overlap.new) }} · нет в файле {{ fmtN(info.overlap.missing) }}</span>
        </div>
      </template>
      <div v-else class="border-t border-slate-800 pt-1.5 text-emerald-400">Новый счёт</div>
    </div>

    <!-- Проблемы и решения -->
    <template v-if="info">
      <div v-if="has('duplicate')" class="ui-banner ui-banner--error">{{ has('duplicate').text }}</div>
      <div v-if="has('other_account')" class="ui-banner ui-banner--error">{{ has('other_account').text }}</div>

      <label v-if="has('exists')" class="flex items-start gap-2 p-2.5 rounded border border-amber-500/40 bg-amber-950/20 text-[11px] text-amber-200 cursor-pointer">
        <input v-model="updateExisting" type="checkbox" class="accent-amber-500 mt-0.5" />
        <span>{{ has('exists').text }} <b>Обновить существующий счёт этим файлом</b></span>
      </label>

      <div v-if="has('alias')" class="p-2.5 rounded border border-slate-700 bg-slate-950/40 text-[11px] space-y-1.5">
        <p class="text-slate-300">{{ has('alias').text }}</p>
        <label class="flex items-center gap-2 cursor-pointer text-slate-200"><input v-model="alias" type="radio" value="keep" class="accent-emerald-500" /> Оставить «{{ info.existing_alias }}»</label>
        <label class="flex items-center gap-2 cursor-pointer text-slate-200"><input v-model="alias" type="radio" value="replace" class="accent-emerald-500" /> Принять «{{ info.alias }}»</label>
      </div>

      <label v-if="has('no_match')" class="flex items-start gap-2 p-2.5 rounded border border-amber-500/40 bg-amber-950/20 text-[11px] text-amber-200 cursor-pointer">
        <input v-model="force" type="checkbox" class="accent-amber-500 mt-0.5" />
        <span>{{ has('no_match').text }} <b>Всё равно записать</b></span>
      </label>
    </template>

    <!-- Статус операции -->
    <div v-if="statusMsg" class="ui-banner break-words" :class="statusMsg.type === 'success' ? 'ui-banner--success' : 'ui-banner--error'">
      {{ statusMsg.text }}
    </div>

    <!-- Кнопки управления -->
    <div class="flex items-center justify-end gap-2 pt-2">
      <UiButton variant="secondary" @click="emit('close')">Отмена</UiButton>
      <UiButton variant="primary" size="lg" :disabled="!canSubmit" @click="submitImport">{{ submitLabel }}</UiButton>
    </div>
  </UiModal>
</template>

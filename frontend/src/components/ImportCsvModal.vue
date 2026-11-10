<script setup>
import { ref, watch } from 'vue'
import { API_BASE } from '../utils/api.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'

const props = defineProps({
  isOpen: { type: Boolean, default: false }
})

const emit = defineEmits(['close', 'success'])

const file = ref(null)
const uploading = ref(false)
const statusMsg = ref(null)
const fileInputRef = ref(null)

watch(
  () => props.isOpen,
  (val) => {
    if (val) {
      file.value = null
      statusMsg.value = null
    }
  }
)

const handleDrop = (e) => {
  e.preventDefault()
  if (e.dataTransfer.files && e.dataTransfer.files[0]) {
    const f = e.dataTransfer.files[0]
    const name = f.name.toLowerCase()
    if (name.endsWith('.csv') || name.endsWith('.pdf')) {
      file.value = f
      statusMsg.value = null
    } else {
      statusMsg.value = { type: 'error', text: 'Выбери файл в формате .csv или .pdf' }
    }
  }
}

const handleFileSelect = (e) => {
  if (e.target.files && e.target.files[0]) {
    file.value = e.target.files[0]
    statusMsg.value = null
  }
}

const submitImport = async () => {
  if (!file.value) return
  uploading.value = true
  statusMsg.value = null

  const formData = new FormData()
  formData.append('file', file.value)

  try {
    const res = await fetch(`${API_BASE}/sync/import-csv`, {
      method: 'POST',
      body: formData
    })

    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || 'Ошибка импорта')
    }

    const data = await res.json()
    statusMsg.value = {
      type: 'success',
      text: `Успешно загружено ${data.imported_count || 0} транзакций!`
    }

    setTimeout(() => {
      emit('success')
      emit('close')
    }, 1200)
  } catch (err) {
    statusMsg.value = { type: 'error', text: err.message }
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-md" @close="emit('close')">
    <!-- Шапка -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span class="text-amber-400 font-bold text-sm">📄</span>
        <span class="text-sm font-bold text-white uppercase tracking-wider">ИМПОРТ ВЫПИСКИ (CSV / PDF)</span>
      </div>
      <UiIconButton icon="close" title="Закрыть" @click="emit('close')" />
    </div>

    <p class="text-[10px] text-slate-500 leading-relaxed">
      Алиас владельца берётся из имени файла (например «Петро Іваненко.csv», «Ivanenko Petro.pdf»).
    </p>

    <!-- Зона Drag and Drop -->
    <div
      @dragover.prevent
      @drop="handleDrop"
      @click="fileInputRef?.click()"
      class="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-all bg-slate-950/50"
      :class="file ? 'border-amber-500/80 bg-amber-950/10' : 'border-slate-800 hover:border-slate-600'"
    >
      <input ref="fileInputRef" type="file" accept=".csv,.pdf" class="hidden" @change="handleFileSelect" />

      <div v-if="file" class="space-y-1">
        <span class="text-xl">📋</span>
        <p class="font-bold text-amber-400 truncate">{{ file.name }}</p>
        <p class="text-[10px] text-slate-500">{{ (file.size / 1024).toFixed(1) }} KB</p>
      </div>
      <div v-else class="space-y-2">
        <span class="text-2xl text-slate-400">📥</span>
        <p class="text-slate-300 font-bold">Перетащи `.csv` или `.pdf` файл сюда</p>
        <p class="text-[10px] text-slate-500">или кликни для выбора</p>
      </div>
    </div>

    <!-- Статус операции -->
    <div v-if="statusMsg" class="ui-banner" :class="statusMsg.type === 'success' ? 'ui-banner--success' : 'ui-banner--error'">
      {{ statusMsg.text }}
    </div>

    <!-- Кнопки управления -->
    <div class="flex items-center justify-end gap-2 pt-2">
      <UiButton variant="secondary" @click="emit('close')">Отмена</UiButton>
      <UiButton variant="amber" size="lg" :disabled="!file || uploading" @click="submitImport">
        {{ uploading ? 'Загрузка...' : 'Загрузить' }}
      </UiButton>
    </div>
  </UiModal>
</template>

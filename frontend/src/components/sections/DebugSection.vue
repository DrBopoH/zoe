<script setup>
import { ref } from 'vue'
import ImportCsvModal from '../ImportCsvModal.vue'
import UiButton from '../ui/UiButton.vue'
import UiGroup from '../ui/UiGroup.vue'
import { apiSend } from '../../utils/api.js'

const emit = defineEmits(['refresh'])

const isImportOpen = ref(false)
const busy = ref(false)
const loading = ref(false)
const loadInfo = ref(null) // { ok, text } — итог последней «Загрузки»

const wipeCache = async () => {
  busy.value = true
  try {
    await apiSend('POST', '/sync/wipe-cache')
    loadInfo.value = null
    emit('refresh')
  } catch (err) {
    console.error('Wipe cache error:', err)
  } finally {
    busy.value = false
  }
}

// Кэш источников (файлы выписок на диске) → в цикле разобрать каждый и записать в базу
const loadSources = async () => {
  loading.value = true
  loadInfo.value = null
  try {
    const res = await apiSend('POST', '/sources/load')
    const d = await res.json()
    const bad = d.failed?.length || 0
    loadInfo.value = {
      ok: bad === 0,
      text: d.total === 0
        ? 'Кэш источников пуст'
        : `Загружено ${d.loaded} из ${d.total}` + (bad ? ` · ошибок: ${bad}` : '')
    }
    if (bad) console.error('Sources load failures:', d.failed)
    emit('refresh')
  } catch (err) {
    loadInfo.value = { ok: false, text: err.message }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <UiGroup title="Кэш">
      <div class="space-y-2">
        <UiButton variant="amber-outline" class="w-full" @click="isImportOpen = true">Импорт</UiButton>
        <div class="flex gap-2">
          <UiButton variant="danger" class="flex-1" :disabled="busy || loading" @click="wipeCache">Сбросить</UiButton>
          <UiButton variant="primary" class="flex-1" :disabled="busy || loading" @click="loadSources">
            {{ loading ? 'Загрузка…' : 'Загрузить' }}
          </UiButton>
        </div>
        <p v-if="loadInfo" class="text-[10px] leading-snug" :class="loadInfo.ok ? 'text-emerald-400' : 'text-rose-400'">
          {{ loadInfo.text }}
        </p>
      </div>
    </UiGroup>

    <ImportCsvModal :is-open="isImportOpen" @close="isImportOpen = false" @success="emit('refresh')" />
  </div>
</template>

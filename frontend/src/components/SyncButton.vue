<script setup>
import { inject } from 'vue'
import UiButton from './ui/UiButton.vue'
import { isSyncing } from '../composables/useSync.js'

// Кнопка «РАЗВЕДАТЬ»: одна на все места (вкладка Смета, окно «Выписки»). Запуск и обработку ошибок делает App (startSync).
defineProps({ size: { type: String, default: 'lg' } })
const startSync = inject('startSync', () => {})
</script>

<template>
  <UiButton variant="primary" :size="size" :disabled="isSyncing" @click="startSync">
    <template v-if="isSyncing">РАЗВЕДЫВАЮ...</template>
    <template v-else>РАЗВЕДАТЬ</template>
  </UiButton>
</template>

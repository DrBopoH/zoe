<script setup>
import UiButton from './UiButton.vue'

// Плашка «раздел не загрузился»: текст ошибки и кнопка повтора. Показывается вместо тихой пустоты.
defineProps({
  message: { type: String, required: true },
  title: { type: String, default: 'Не удалось загрузить раздел' }
})
defineEmits(['retry'])
</script>

<template>
  <div role="alert" class="flex flex-wrap items-center gap-3 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-xs">
    <div class="min-w-0 flex-1">
      <div class="font-bold uppercase tracking-wider text-rose-300">{{ title }}</div>
      <div class="break-words text-rose-200/80">{{ message }}</div>
    </div>
    <UiButton variant="outline" size="sm" @click="$emit('retry')">Повторить</UiButton>
  </div>
</template>

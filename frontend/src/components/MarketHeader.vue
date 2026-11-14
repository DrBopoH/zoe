<script setup>
import UiButton from './ui/UiButton.vue'

// Шапка раздела «Рынок»: заголовок, справа кнопка «Обновить». Всё своё у раздела (поиск, переключатели, «последняя точка …»)
// кладётся в слот между ними. Кнопка сама не знает, что обновлять: нажатие уходит событием refresh.
defineProps({
  title: { type: String, required: true },
  running: { type: Boolean, default: false } // идёт опрос источников: кнопка занята
})
defineEmits(['refresh'])
</script>

<template>
  <section class="flex flex-wrap items-center gap-3 text-xs">
    <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">{{ title }}</h2>
    <slot />
    <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="$emit('refresh')">
      <template v-if="running">ОБНОВЛЯЮ...</template>
      <template v-else>ОБНОВИТЬ</template>
    </UiButton>
  </section>
</template>

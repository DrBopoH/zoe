<script setup>
import UiButton from './ui/UiButton.vue'
import UiIcon from './ui/UiIcon.vue'

// Шапка раздела «Рынок»: заголовок, справа кнопка обновления — иконка из двух стрелок: при наведении поворачивается, пока идёт опрос — серая кнопка с крутящимися стрелками. Всё своё у раздела (поиск, переключатели, «последняя точка …»)
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
    <UiButton
      variant="secondary" size="sm" class="ml-auto group px-2!" :disabled="running" :aria-busy="running"
      :aria-label="running ? 'Обновляю…' : 'Обновить'" :title="running ? 'Обновляю…' : 'Обновить'" @click="$emit('refresh')"
    >
      <UiIcon
        name="refresh" size="lg"
        :class="running ? 'animate-spin text-slate-500' : 'transition-transform duration-500 ease-out group-hover:rotate-180'"
        class="motion-reduce:animate-none motion-reduce:transition-none"
      />
    </UiButton>
  </section>
</template>

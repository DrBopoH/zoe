<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import UiButton from './ui/UiButton.vue'
import UiIcon from './ui/UiIcon.vue'

// Шапка раздела «Рынок»: заголовок, справа кнопка обновления — иконка из двух стрелок: при наведении поворачивается, пока идёт опрос — серая кнопка с крутящимися стрелками. Всё своё у раздела (поиск, переключатели, «последняя точка …»)
// кладётся в слот между ними. Кнопка сама не знает, что обновлять: нажатие уходит событием refresh.
const props = defineProps({
  title: { type: String, required: true },
  running: { type: Boolean, default: false } // идёт опрос источников: кнопка занята
})
const emit = defineEmits(['refresh'])

// Статус опроса приходит раз в несколько секунд, а быстрый источник отрабатывает за одну: без этого кнопка не успевала бы ни провернуться, ни стать серой.
// Поэтому после нажатия она занята сама минимум KICK_MS, а дальше — пока статус говорит «идёт».
const KICK_MS = 3000
const kicked = ref(false)
let kickTimer = null
const busy = computed(() => props.running || kicked.value)
const press = () => {
  kicked.value = true
  clearTimeout(kickTimer)
  kickTimer = setTimeout(() => { kicked.value = false }, KICK_MS)
  emit('refresh')
}
onBeforeUnmount(() => clearTimeout(kickTimer))
</script>

<template>
  <section class="flex flex-wrap items-center gap-3 text-xs">
    <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">{{ title }}</h2>
    <slot />
    <UiButton
      variant="secondary" size="sm" class="ml-auto group px-2!" :disabled="busy" :aria-busy="busy"
      :aria-label="busy ? 'Обновляю…' : 'Обновить'" :title="busy ? 'Обновляю…' : 'Обновить'" @click="press"
    >
      <UiIcon
        name="refresh" size="lg"
        :class="busy ? 'animate-spin text-slate-500' : 'transition-transform duration-500 ease-out group-hover:rotate-180'"
      />
    </UiButton>
  </section>
</template>

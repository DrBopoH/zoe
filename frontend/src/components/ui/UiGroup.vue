<script setup>
// Абзац внутри раздела с мелким серым заголовком слева сверху.
// Контур у раздела один (его рисует SideMenu), группы идут подряд без перегородок:
// КЭШ, ОБЩЕЕ, СВЕДЕНИЯ, ИНФОРМАЦИЯ… — любое число.
defineProps({
  title: { type: String, default: '' }
})
</script>

<template>
  <section class="ui-group">
    <h3 v-if="title" class="ui-group__title">{{ title }}</h3>
    <slot />
  </section>
</template>
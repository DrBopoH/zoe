<script setup>
// Карточка-контейнер макро-экранов: рамка, тёмная подложка, скругление. Отступы, размер текста и прокрутку
// родитель добавляет своим class (p-3, text-xs, overflow-x-auto, divide-y…) — они складываются с базовыми.
defineProps({
  as: { type: String, default: 'section' }
})
</script>

<template>
  <component :is="as" class="rounded-lg border border-slate-800 bg-slate-900/40">
    <slot />
  </component>
</template>

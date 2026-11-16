<script setup>
// Строка-предупреждение над блоком (например, «курса доллара пока нет»): янтарная рамка, короткий текст.
defineProps({
  tone: { type: String, default: 'warn' } // warn
})
</script>

<template>
  <div class="rounded-lg border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-300" :data-tone="tone">
    <slot />
  </div>
</template>

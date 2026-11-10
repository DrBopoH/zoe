<script setup>
import UiIconButton from './UiIconButton.vue'

defineProps({
  name: { type: String, required: true },
  cached: { type: Boolean, default: false },
  sub: { type: Boolean, default: false },   // «другой» алиас: контакт, делящий счёт с главным
  loose: { type: Boolean, default: false }  // алиас без счетов
})
const emit = defineEmits(['edit', 'unlink'])
</script>

<template>
  <span
    class="ui-chip"
    :class="[
      cached ? 'ui-chip--cached' : 'ui-chip--static',
      loose && 'ui-chip--loose',
      !loose && (sub ? 'ui-chip--sub' : 'ui-chip--main')
    ]"
  >
    <span v-if="sub" class="text-slate-500 select-none text-[10px] font-bold">└─</span>
    <span>{{ name }}</span>
    <UiIconButton icon="edit" tone="accent" size="sm" title="Редактировать связи" @mousedown.stop @click.stop.prevent="emit('edit')" />
    <UiIconButton icon="close" tone="danger" size="sm" title="Отвязать от лица" @mousedown.stop @click.stop.prevent="emit('unlink')" />
  </span>
</template>

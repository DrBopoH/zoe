<script setup>
import UiIcon from './UiIcon.vue'

defineProps({
  icon: { type: String, required: true },
  tone: { type: String, default: 'muted' }, // muted | accent | danger
  size: { type: String, default: 'md' }     // sm | md
})
</script>

<template>
  <button
    type="button"
    class="ui-icon-btn"
    :class="[
      tone === 'accent' && 'ui-icon-btn--accent',
      tone === 'danger' && 'ui-icon-btn--danger',
      size === 'sm' && 'ui-icon-btn--sm'
    ]"
  >
    <UiIcon :name="icon" :size="size" />
  </button>
</template>

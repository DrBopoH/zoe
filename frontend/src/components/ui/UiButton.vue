<script setup>
defineProps({
  variant: { type: String, default: 'primary' }, // primary | secondary | danger | amber | subtle | cancel | outline
  size: { type: String, default: 'md' }          // sm | md | lg
})
</script>

<template>
  <button type="button" class="ui-btn" :class="[`ui-btn--${variant}`, `ui-btn--${size}`]">
    <slot />
  </button>
</template>

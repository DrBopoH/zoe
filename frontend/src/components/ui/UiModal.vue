<script setup>
defineProps({
  isOpen: { type: Boolean, default: false },
  maxWidth: { type: String, default: 'max-w-md' } // любой tailwind max-w-*
})
const emit = defineEmits(['close'])
</script>

<template>
  <Transition name="fade">
    <div v-if="isOpen" class="ui-overlay" data-modal @click.self="emit('close')">
      <div class="ui-panel" :class="maxWidth">
        <slot />
      </div>
    </div>
  </Transition>
</template>

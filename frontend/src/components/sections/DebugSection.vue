<script setup>
import { ref } from 'vue'
import StatementsModal from '../StatementsModal.vue'
import UiButton from '../ui/UiButton.vue'
import UiGroup from '../ui/UiGroup.vue'

const emit = defineEmits(['refresh'])
const open = ref(false)
</script>

<template>
  <div>
    <UiGroup title="Кэш">
      <UiButton variant="amber-outline" class="w-full" @click="open = true">Выписки</UiButton>
    </UiGroup>

    <StatementsModal :is-open="open" @close="open = false" @refresh="emit('refresh')" />
  </div>
</template>

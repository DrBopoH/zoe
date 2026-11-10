<script setup>
// Зелёный = подтверждено кэшем, жёлтый = помнит только статика
defineProps({ cached: { type: Boolean, default: false } })
</script>

<template>
  <span class="ui-dot" :class="cached ? 'ui-dot--cached' : 'ui-dot--static'"></span>
</template>

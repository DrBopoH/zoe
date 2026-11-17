<script setup>
import { computed } from 'vue'
import { archiveLabel } from '../utils/nav.js'

const props = defineProps({
  activeTab: { type: String, required: true },
  isSyncing: { type: Boolean, default: false }
})

const page = computed(() => archiveLabel(props.activeTab))
</script>

<template>
  <header class="flex items-center gap-4">
    <div class="flex items-center gap-2">
      <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" :class="{ 'animate-pulse': isSyncing }"></span>
      <span class="text-sm font-bold tracking-wider text-white">Ζωή</span>
    </div>
    <div class="flex items-center gap-2 border-l border-slate-800 pl-4 text-xs uppercase font-bold">
      <span class="text-slate-500">Архив</span>
      <span class="text-slate-700">/</span>
      <span class="text-emerald-400">{{ page }}</span>
    </div>
  </header>
</template>

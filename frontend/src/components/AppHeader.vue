<script setup>
defineProps({
  activeTab: { type: String, required: true },
  isSyncing: { type: Boolean, default: false }
})

const emit = defineEmits(['update:activeTab'])

const TABS = [
  { key: 'dashboard', label: 'Смета' },
  { key: 'data', label: 'Данные' }
]
</script>

<template>
  <header class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div class="flex items-center gap-6">
      <div class="flex items-center gap-2">
        <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" :class="{ 'animate-pulse': isSyncing }"></span>
        <span class="text-sm font-bold tracking-wider text-white">ECONOMIC</span>
      </div>

      <nav class="flex items-center gap-1 border-l border-slate-800 pl-6 text-xs">
        <button
          v-for="tab in TABS"
          :key="tab.key"
          @click="emit('update:activeTab', tab.key)"
          class="px-3 py-1.5 rounded transition-colors uppercase font-bold cursor-pointer flex items-center gap-1.5"
          :class="activeTab === tab.key ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
        >
          {{ tab.label }}
        </button>
      </nav>
    </div>
  </header>
</template>

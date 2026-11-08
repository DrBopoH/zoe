<script setup>
defineProps({
  activeTab: { type: String, required: true },
  isSyncing: { type: Boolean, default: false },
  syncStatusMsg: { type: String, default: 'IDLE' },
  personsCount: { type: Number, default: 0 }
})

const emit = defineEmits(['update:activeTab', 'sync'])
</script>

<template>
  <header class="border-b border-slate-800 pb-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div class="flex items-center gap-6">
      <div class="flex items-center gap-2">
        <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" :class="{ 'animate-pulse': isSyncing }"></span>
        <span class="text-sm font-bold tracking-wider text-white">ECONOMIC</span>
      </div>

      <nav class="flex items-center gap-1 border-l border-slate-800 pl-6 text-xs">
        <button
          @click="emit('update:activeTab', 'dashboard')"
          class="px-3 py-1.5 rounded transition-colors uppercase font-bold"
          :class="activeTab === 'dashboard' ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
        >
          Смета
        </button>
        <button
          @click="emit('update:activeTab', 'accounts')"
          class="px-3 py-1.5 rounded transition-colors uppercase font-bold"
          :class="activeTab === 'accounts' ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
        >
          Операционная
        </button>
        <button
          @click="emit('update:activeTab', 'entities')"
          class="px-3 py-1.5 rounded transition-colors uppercase font-bold flex items-center gap-1.5"
          :class="activeTab === 'entities' ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
        >
          Лица
          <span class="text-[9px] px-1.5 py-0.2 bg-slate-900 border border-slate-700 rounded text-slate-400">{{ personsCount }}</span>
        </button>
      </nav>
    </div>

    <div class="flex items-center gap-4">
      <span class="text-[11px] text-slate-500 uppercase">{{ syncStatusMsg }}</span>
      <button
        @click="emit('sync')"
        :disabled="isSyncing"
        class="bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-800 disabled:text-slate-600 text-white font-bold text-xs px-4 py-1.5 rounded transition-all cursor-pointer uppercase"
      >
        {{ isSyncing ? 'SYNCING...' : 'SYNC' }}
      </button>
    </div>
  </header>
</template>
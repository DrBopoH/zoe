<script setup>
import SideMenu from './SideMenu.vue'
import SyncButton from './SyncButton.vue'
import { HOME_TAB, CASES_TAB } from '../utils/nav.js'

defineProps({
  activeTab: { type: String, required: true },
  isSyncing: { type: Boolean, default: false }
})

defineEmits(['update:activeTab', 'refresh'])
</script>

<template>
  <header class="flex items-center gap-3">
    <SideMenu :active-tab="activeTab" @update:active-tab="$emit('update:activeTab', $event)" @refresh="$emit('refresh')" />
    <button type="button" class="flex items-center gap-2 cursor-pointer" title="Главная" @click="$emit('update:activeTab', HOME_TAB)">
      <span class="w-2.5 h-2.5 rounded-full bg-emerald-500" :class="{ 'animate-pulse': isSyncing }"></span>
      <span class="text-sm font-bold tracking-wider text-white">Ζωή</span>
    </button>
    <button
      type="button"
      class="px-3 py-1.5 rounded transition-colors uppercase text-xs font-bold cursor-pointer"
      :class="activeTab === CASES_TAB ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
      @click="$emit('update:activeTab', CASES_TAB)"
    >Дела</button>
    <SyncButton size="md" class="ml-auto" />
  </header>
</template>

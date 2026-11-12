<script setup>
import { ref, watch, nextTick } from 'vue'
import OperationalView from './OperationalView.vue'
import PersonsView from './PersonsView.vue'
import { storageGet, storageSet } from '../utils/storage.js'
import { resolveNav } from '../utils/nav.js'

defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] },
  dataVersion: { default: null },
  personsCount: { type: Number, default: 0 }
})
const emit = defineEmits(['refresh'])

const STORAGE_SECTION = 'economic_data_section'
const SECTIONS = [
  { key: 'history', label: 'История' },
  { key: 'persons', label: 'Лица', counter: true }
]

const section = ref(resolveNav('data', storageGet(STORAGE_SECTION)).section)
watch(section, (s) => storageSet(STORAGE_SECTION, s))

// Как и у верхних вкладок: у каждого раздела своя позиция прокрутки страницы
const scrollOf = {}
watch(section, (_, old) => { if (old) scrollOf[old] = window.scrollY }, { flush: 'pre' })
watch(section, async (s) => {
  await nextTick()
  window.scrollTo(0, scrollOf[s] || 0)
})
</script>

<template>
  <div class="flex flex-col md:flex-row gap-6 items-start">
    <nav class="flex md:flex-col gap-1 text-xs md:w-36 shrink-0 md:sticky md:top-20" aria-label="Разделы данных">
      <button
        v-for="s in SECTIONS"
        :key="s.key"
        type="button"
        @click="section = s.key"
        class="px-3 py-2 rounded transition-colors uppercase font-bold cursor-pointer flex items-center justify-between gap-2 text-left"
        :class="section === s.key ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
      >
        {{ s.label }}
        <span v-if="s.counter" class="text-[9px] px-1.5 bg-slate-900 border border-slate-700 rounded text-slate-400">{{ personsCount }}</span>
      </button>
    </nav>

    <div class="min-w-0 flex-1 w-full">
      <KeepAlive><OperationalView v-if="section === 'history'" :accounts="accounts" :transactions="transactions" :users="users" /></KeepAlive>
      <KeepAlive><PersonsView v-if="section === 'persons'" :transactions="transactions" :accounts="accounts" :data-version="dataVersion" @refresh="emit('refresh')" /></KeepAlive>
    </div>
  </div>
</template>

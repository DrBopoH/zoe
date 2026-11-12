<script setup>
import { ref, watch, nextTick } from 'vue'
import UiIcon from '../components/ui/UiIcon.vue'
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
  { key: 'history', label: 'История', icon: 'history' },
  { key: 'persons', label: 'Лица', icon: 'users', counter: true }
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
  <!-- Список разделов стоит ЗА основной сеткой (в левом поле), чтобы не сдвигать и не сужать экран: тот же размер, что у Сметы.
       Широкое поле (окно от 1380px) — подписи; среднее (от 1240px) — узкая колонка иконок; на узком окне — строка сверху. -->
  <div class="relative">
    <div class="min-[1240px]:absolute min-[1240px]:right-full min-[1240px]:top-0 min-[1240px]:bottom-0 min-[1240px]:mr-2 min-[1240px]:w-9 min-[1380px]:mr-3 min-[1380px]:w-24 mb-4 min-[1240px]:mb-0">
      <nav class="flex min-[1240px]:flex-col gap-1 text-xs min-[1240px]:sticky min-[1240px]:top-20" aria-label="Разделы учета">
        <button
          v-for="s in SECTIONS"
          :key="s.key"
          type="button"
          :title="s.counter ? `${s.label} · ${personsCount}` : s.label"
          @click="section = s.key"
          class="px-3 py-2 min-[1240px]:px-0 min-[1380px]:px-3 rounded transition-colors uppercase font-bold cursor-pointer flex items-center justify-between min-[1240px]:justify-center min-[1380px]:justify-between gap-2 text-left"
          :class="section === s.key ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
        >
          <UiIcon :name="s.icon" size="lg" class="hidden min-[1240px]:block min-[1380px]:hidden" />
          <span class="min-[1240px]:hidden min-[1380px]:inline">{{ s.label }}</span>
          <span v-if="s.counter" class="text-[9px] px-1.5 bg-slate-900 border border-slate-700 rounded text-slate-400 min-[1240px]:hidden min-[1380px]:inline">{{ personsCount }}</span>
        </button>
      </nav>
    </div>

    <KeepAlive><OperationalView v-if="section === 'history'" :accounts="accounts" :transactions="transactions" :users="users" /></KeepAlive>
    <KeepAlive><PersonsView v-if="section === 'persons'" :transactions="transactions" :accounts="accounts" :data-version="dataVersion" @refresh="emit('refresh')" /></KeepAlive>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { formatRuTime, formatDuration } from '../utils/formatters.js'
import UiIcon from './ui/UiIcon.vue'
import SyncTimeline from './SyncTimeline.vue'

// Главный пуш — состояние «processing» (висит, пока идёт синк) или «error» (висит, пока не закроют).
// Слева сверху, не скрываясь: прикидка времени окончания. Справа — крестик, он глушит уведомление.
// Стрелка у клиента сворачивает счета до общего бара с процентом.
const props = defineProps({
  progress: { type: Object, required: true },
  state: { type: String, default: 'processing' } // processing | error
})
const emit = defineEmits(['close'])

const now = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => clearInterval(timer))

const collapsed = ref({}) // clientKey → true
const keyOf = (c) => c.kind + ':' + c.name
const toggle = (c) => { collapsed.value = { ...collapsed.value, [keyOf(c)]: !collapsed.value[keyOf(c)] } }

const isErr = (c) => !!c.error || c.accounts.some(a => a.state === 'error')
const isDone = (c) => c.finished && !isErr(c) && c.percent >= 100

const eta = computed(() => {
  const t = props.progress.eta
  if (!t) return null
  return { at: formatRuTime(t), in: formatDuration(t - now.value / 1000) }
})
</script>

<template>
  <div class="ui-push ui-push--proc" :class="state === 'error' && 'ui-push--err'">
    <!-- нескрываемая шапка: ETA слева, крестик справа -->
    <div class="flex items-center justify-between gap-3">
      <div v-if="state === 'error'" class="flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider text-rose-300 min-w-0">
        <span class="w-1.5 h-1.5 rounded-full bg-rose-400 shrink-0"></span>Синк завершён с ошибками
      </div>
      <div v-else class="flex items-center gap-1.5 text-[10px] text-slate-300 min-w-0">
        <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse shrink-0"></span>
        <template v-if="eta">
          <span class="uppercase tracking-wider text-slate-500">Закончится</span>
          <span class="font-bold text-slate-100">≈ {{ eta.at }}</span>
          <span class="text-slate-500 truncate">· через {{ eta.in }}</span>
        </template>
        <span v-else class="uppercase tracking-wider text-slate-500">Считаю время…</span>
      </div>
      <button type="button" class="text-slate-400 hover:text-white cursor-pointer shrink-0" aria-label="Закрыть" @click="emit('close')">
        <UiIcon name="close" size="sm" />
      </button>
    </div>

    <div v-if="progress.clients.length === 0" class="mt-3 ui-bar ui-bar--run"></div>

    <!-- список клиентов прокручивается: шапка с временем и крестиком остаётся на месте -->
    <div class="mt-1 max-h-[calc(100vh-11rem)] overflow-y-auto overscroll-contain pr-2 -mr-1">
    <div v-for="c in progress.clients" :key="keyOf(c)" class="mt-3 space-y-2">
      <button type="button" class="w-full flex items-center gap-2 text-left cursor-pointer" @click="toggle(c)">
        <UiIcon :name="collapsed[keyOf(c)] ? 'arrowDown' : 'arrowUp'" size="sm" class="text-slate-500 shrink-0" />
        <span class="flex-1 min-w-0 truncate text-xs font-bold text-slate-100">{{ c.name }}</span>
        <UiIcon v-if="isDone(c)" name="check" size="sm" class="text-emerald-400" />
        <span v-else class="text-xs font-bold tabular-nums" :class="isErr(c) ? 'text-rose-400' : 'text-emerald-400'">{{ c.percent }}%</span>
      </button>

      <div v-if="c.error" class="pl-6 text-[10px] text-rose-400">{{ c.error }}</div>

      <!-- свёрнуто: общий бар — бегущие полосы, пока идёт; заливка, когда готово / ошибка -->
      <div v-if="collapsed[keyOf(c)]" class="pl-6">
        <div class="ui-bar" :class="isErr(c) ? 'ui-bar--err' : isDone(c) ? 'ui-bar--done' : 'ui-bar--run'"></div>
      </div>

      <!-- развёрнуто: счета с таймлайнами -->
      <div v-else class="pl-6 space-y-3">
        <div v-for="a in c.accounts" :key="a.id" class="space-y-1">
          <div class="text-[10px] text-slate-400 tracking-wide truncate">{{ a.label }}</div>
          <SyncTimeline :acc="a" :now="now" />
        </div>
      </div>
    </div>
    </div>
  </div>
</template>

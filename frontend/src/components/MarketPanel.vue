<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { feedView, elapsedText } from '../utils/marketPanel.js'
import UiIcon from './ui/UiIcon.vue'

// Окно процесса рыночных данных (ОВДП НБУ, US Treasury, дальше макро): висит, пока идёт опрос, как окно синка банков,
// но это отдельный процесс со своими строками. Итог по каждому источнику потом показывают зелёные/красные тосты.
// Источник с известным процентом (длинные выгрузки) рисует заливку, без процента — бегущие полосы.
const props = defineProps({
  market: { type: Object, required: true } // { running, started_at, feeds: [{ id, title, state, pct, note, fetched, error }] }
})
const emit = defineEmits(['close'])

const now = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="ui-push ui-push--proc">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-1.5 text-[10px] text-slate-300 min-w-0">
        <span class="w-1.5 h-1.5 rounded-full bg-sky-400 animate-pulse shrink-0"></span>
        <span class="uppercase tracking-wider text-slate-500">Рыночные данные</span>
        <span class="font-bold text-slate-100 tabular-nums">{{ elapsedText(market.started_at, now) }}</span>
      </div>
      <button type="button" class="text-slate-400 hover:text-white cursor-pointer shrink-0" aria-label="Закрыть" @click="emit('close')">
        <UiIcon name="close" size="sm" />
      </button>
    </div>

    <div v-for="f in market.feeds" :key="f.id" class="mt-3 space-y-1.5">
      <div class="flex items-center gap-2">
        <span class="flex-1 min-w-0 truncate text-xs font-bold text-slate-100">{{ f.title }}</span>
        <UiIcon v-if="feedView(f).mode === 'done'" name="check" size="sm" class="text-emerald-400" />
        <span v-else-if="feedView(f).mode === 'pct'" class="text-xs font-bold tabular-nums text-emerald-400">{{ feedView(f).pct }}%</span>
        <span v-else-if="feedView(f).mode === 'error'" class="text-xs font-bold text-rose-400">ошибка</span>
      </div>
      <div class="pl-0">
        <div v-if="feedView(f).mode === 'pct'" class="ui-bar"><div class="h-full bg-emerald-500 transition-all duration-500" :style="{ width: feedView(f).pct + '%' }"></div></div>
        <div v-else class="ui-bar" :class="{ 'ui-bar--run': feedView(f).mode === 'run', 'ui-bar--done': feedView(f).mode === 'done', 'ui-bar--err': feedView(f).mode === 'error' }"></div>
      </div>
      <div v-if="feedView(f).text" class="text-[10px] break-words" :class="feedView(f).mode === 'error' ? 'text-rose-400' : 'text-slate-400'">{{ feedView(f).text }}</div>
    </div>
  </div>
</template>

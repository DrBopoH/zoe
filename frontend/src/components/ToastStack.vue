<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { toasts, dismissToast } from '../composables/useToasts.js'
import SyncPanel from './SyncPanel.vue'
import MarketPanel from './MarketPanel.vue'
import { scrollMode, scrollTarget } from '../utils/stackScroll.js'
import UiIcon from './ui/UiIcon.vue'

// Стек уведомлений справа внизу. У всех — крестик.
//   processing — главное окно, висит, пока идёт синк
//   error      — то же окно, красное, не пропадает само
//   market     — окно процесса рыночных данных (ОВДП, US Treasury): висит, пока идёт опрос
//   push       — «Снято»: появляется и исчезает по таймеру
//   lost       — нет ответа от бэкенда: синк мог продолжать работу, статус вернётся вместе со связью
const props = defineProps({
  progress: { type: Object, default: null },
  panelState: { type: String, default: null }, // null | processing | error
  market: { type: Object, default: null },     // состояние рыночного процесса, если окно открыто
  lostSince: { type: Number, default: null }   // ms, когда пропал ответ бэкенда
})
const emit = defineEmits(['close-panel', 'close-market'])

const now = ref(Date.now())
let timer = 0
watch(() => props.lostSince, (v) => {
  clearInterval(timer)
  if (v) { now.value = Date.now(); timer = setInterval(() => { now.value = Date.now() }, 1000) }
}, { immediate: true })

// Стопка прокручивается, когда не помещается. Режим (у начала / у конца / посередине) запоминается по scroll и
// восстанавливается после каждого изменения размера (появление, уход, анимации): ползунок не «плывёт».
const stack = ref(null)
let mode = 'top'
let ro = null
let mo = null
const onScroll = () => { const el = stack.value; if (el) mode = scrollMode(el.scrollTop, el.scrollHeight, el.clientHeight) }
const restore = () => {
  const el = stack.value
  if (!el) return
  const to = scrollTarget(mode, el.scrollHeight, el.clientHeight)
  if (to !== null && el.scrollTop !== to) el.scrollTop = to
}
const watchChildren = () => {
  const el = stack.value
  if (!el || !ro) return
  ro.disconnect()
  ro.observe(el)
  for (const c of el.children) ro.observe(c)
}
onMounted(() => {
  const el = stack.value
  if (!el || typeof ResizeObserver === 'undefined') return
  ro = new ResizeObserver(restore)
  mo = new MutationObserver(() => { watchChildren(); restore() })
  mo.observe(el, { childList: true })
  watchChildren()
})
onBeforeUnmount(() => { clearInterval(timer); ro?.disconnect(); mo?.disconnect() })

const dismissedLost = ref(null) // какой разрыв связи закрыт крестиком
const showLost = computed(() => !!props.lostSince && dismissedLost.value !== props.lostSince)
const lostFor = computed(() => {
  const s = Math.max(0, Math.round((now.value - props.lostSince) / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
})

// Новые тосты появляются сверху, старые копятся внизу: нижний (самый старый) уходит первым
const toastsView = computed(() => [...toasts.value].reverse())
</script>

<template>
  <div ref="stack" class="ui-push-stack" @scroll.passive="onScroll">
    <TransitionGroup name="push">
      <SyncPanel
        v-if="progress && panelState"
        key="panel"
        :progress="progress"
        :state="panelState"
        :class="lostSince && 'opacity-60'"
        @close="emit('close-panel')"
      />

      <MarketPanel v-if="market" key="market" :market="market" @close="emit('close-market')" />

      <div v-if="showLost" key="lost" class="ui-push ui-push--warn">
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1 min-w-0">
            <div class="text-xs font-bold uppercase tracking-wider text-amber-300">Нет ответа от бэкенда · {{ lostFor }}</div>
            <div class="text-[10px] text-slate-300 break-words">Синк мог продолжить работу на своей стороне — статус обновится сам, когда связь вернётся.</div>
          </div>
          <button type="button" class="text-slate-400 hover:text-white cursor-pointer shrink-0" aria-label="Скрыть" @click="dismissedLost = lostSince">
            <UiIcon name="close" size="sm" />
          </button>
        </div>
      </div>

      <div
        v-for="t in toastsView"
        :key="t.id"
        class="ui-push"
        :class="{ 'ui-push--err': t.kind === 'err', 'ui-push--warn': t.kind === 'warn', 'ui-push--ok': t.kind === 'ok' }"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1 min-w-0">
            <div class="text-xs font-bold uppercase tracking-wider" :class="{ 'text-rose-300': t.kind === 'err', 'text-amber-300': t.kind === 'warn', 'text-emerald-300': t.kind === 'ok' }">{{ t.title }}</div>
            <div v-for="(l, i) in t.lines" :key="i" class="text-[10px] text-slate-300 break-words">{{ l }}</div>
          </div>
          <button type="button" class="text-slate-400 hover:text-white cursor-pointer shrink-0" aria-label="Закрыть" @click="dismissToast(t.id)">
            <UiIcon name="close" size="sm" />
          </button>
        </div>
      </div>
    </TransitionGroup>
  </div>
</template>

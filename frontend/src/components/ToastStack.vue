<script setup>
import { toasts, dismissToast } from '../composables/useToasts.js'
import SyncPanel from './SyncPanel.vue'
import UiIcon from './ui/UiIcon.vue'

// Стек уведомлений справа внизу. У всех — крестик.
//   processing — главное окно, висит, пока идёт синк
//   error      — то же окно, красное, не пропадает само
//   push       — «Снято»: появляется и исчезает по таймеру
defineProps({
  progress: { type: Object, default: null },
  panelState: { type: String, default: null } // null | processing | error
})
const emit = defineEmits(['close-panel'])
</script>

<template>
  <div class="ui-push-stack">
    <TransitionGroup name="push">
      <SyncPanel
        v-if="progress && panelState"
        key="panel"
        :progress="progress"
        :state="panelState"
        @close="emit('close-panel')"
      />

      <div
        v-for="t in toasts"
        :key="t.id"
        class="ui-push"
        :class="t.kind === 'err' ? 'ui-push--err' : 'ui-push--ok'"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1 min-w-0">
            <div class="text-xs font-bold uppercase tracking-wider" :class="t.kind === 'err' ? 'text-rose-300' : 'text-emerald-300'">{{ t.title }}</div>
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

<script setup>
import { computed, ref } from 'vue'
import AccountPanel from './AccountPanel.vue'

// Все карты Операционной и PAN-сироты плитками. Тащатся на рядки истории (drag&drop, как плитки в «Лицах»).
const props = defineProps({
  cards: { type: Array, default: () => [] },   // карты кэша
  orphans: { type: Array, default: () => [] }, // PAN-сироты
  aliasesOf: { type: Function, default: () => [] }
})
const emit = defineEmits(['dragstart', 'dragend'])

const q = ref('')
const match = (c) => {
  const s = q.value.replace(/[\s*•]/g, '').toLowerCase()
  if (!s) return true
  return [c.displayIban, ...c.pans, ...c.ids].some(x => String(x).replace(/[\s*•]/g, '').toLowerCase().includes(s))
}
const cardsShown = computed(() => props.cards.filter(match))
const LIMIT = 120
const orphansAll = computed(() => props.orphans.filter(match))
const orphansShown = computed(() => orphansAll.value.slice(0, LIMIT))

const start = (e, c) => {
  e.dataTransfer.effectAllowed = 'link'
  e.dataTransfer.setData('text/plain', c.key)
  emit('dragstart', c)
}
const badge = (c) => {
  const a = props.aliasesOf(c)
  return a.length ? (a.length > 1 ? `${a[0]} +${a.length - 1}` : a[0]) : ''
}
</script>

<template>
  <aside class="tiles">
    <div class="p-2 border-b border-slate-800">
      <input v-model="q" type="text" class="ui-input" placeholder="Поиск по маске: 4149…0172" />
    </div>

    <div class="tiles-scroll">
      <div class="ui-label px-1 pb-1.5">Карты · {{ cardsShown.length }}</div>
      <div class="grid grid-cols-2 gap-2 items-start">
        <div v-for="c in cardsShown" :key="c.key" class="tile" draggable="true" @dragstart="start($event, c)" @dragend="emit('dragend')">
          <AccountPanel compact :card="c" />
          <div v-if="badge(c)" class="tile-badge" :title="aliasesOf(c).join(', ')">{{ badge(c) }}</div>
        </div>
      </div>

      <div class="ui-label px-1 pt-4 pb-1.5">PAN-сироты · {{ orphansAll.length }}</div>
      <div class="grid grid-cols-2 gap-2 items-start">
        <div v-for="c in orphansShown" :key="c.key" class="tile" draggable="true" @dragstart="start($event, c)" @dragend="emit('dragend')">
          <AccountPanel compact :card="c" class="!border-dashed" />
          <div v-if="badge(c)" class="tile-badge" :title="aliasesOf(c).join(', ')">{{ badge(c) }}</div>
        </div>
      </div>
      <div v-if="orphansAll.length > LIMIT" class="text-[10px] text-slate-600 text-center py-2">
        Показаны первые {{ LIMIT }} — уточни поиск
      </div>
    </div>
  </aside>
</template>

<style scoped>
.tiles {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgb(15 23 42);
  border: 1px solid rgb(30 41 59);
  border-radius: 0.5rem;
  overflow: hidden;
}
.tiles-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0.5rem;
  scrollbar-width: thin;
  scrollbar-color: rgb(52 211 153 / 0.5) transparent;
}
.tile { position: relative; cursor: grab; }
.tile:active { cursor: grabbing; }
.tile-badge {
  margin-top: 3px;
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgb(6 78 59 / 0.35);
  border: 1px solid rgb(16 185 129 / 0.5);
  color: rgb(110 231 183);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

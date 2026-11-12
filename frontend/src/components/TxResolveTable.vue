<script setup>
import { computed, ref, watch } from 'vue'
import { moneySigned, curSym } from '../utils/formatters.js'

// История выбранной карты. Рядки-контрагенты — цели для drop карты (привязка к алиасу контрагента).
const props = defineProps({
  rows: { type: Array, default: () => [] },
  dragging: { type: Boolean, default: false },
  compat: { type: Function, default: () => null }, // (row) → true | false | null во время перетаскивания
  empty: { type: String, default: 'Нет операций' }
})
const emit = defineEmits(['drop-row'])

const PAGE = 150
const visible = ref(PAGE)
watch(() => props.rows, () => { visible.value = PAGE })
const shown = computed(() => props.rows.slice(0, visible.value))

const overId = ref(null)
const onOver = (e, r) => {
  if (!props.dragging || !r.isCp) return
  e.preventDefault()
  e.dataTransfer.dropEffect = 'link'
  overId.value = r.id
}
const onDrop = (e, r) => {
  if (!r.isCp) return
  e.preventDefault()
  overId.value = null
  emit('drop-row', r)
}

const money = (t) => moneySigned(t.amount, curSym(t.currency_code))
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')
const pad = (n) => String(n).padStart(2, '0')
const dateStr = (iso) => { const d = new Date(iso); return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}` }
const timeStr = (iso) => { const d = new Date(iso); return `${pad(d.getHours())}:${pad(d.getMinutes())}` }
const bg = (r) => ({ plain: '', yellow: 'tx-yellow', green: 'tx-green' })[r.state]
const dim = (r) => props.dragging && r.isCp && props.compat(r) === false
const lit = (r) => props.dragging && r.isCp && props.compat(r) === true
</script>

<template>
  <div class="tx-scroll">
    <table class="w-full text-left border-collapse text-xs">
      <thead class="sticky top-0 z-10 bg-slate-950">
        <tr class="text-[10px] text-slate-500 uppercase tracking-wider border-b border-slate-800">
          <th class="px-3 h-8 font-normal w-[96px]">Дата</th>
          <th class="px-2 font-normal">Контрагент</th>
          <th class="px-2 font-normal w-[190px]">Счета</th>
          <th class="px-3 font-normal text-right w-[120px]">Сумма</th>
        </tr>
      </thead>
      <tbody class="font-mono">
        <tr
          v-for="r in shown"
          :key="r.id"
          class="tx-row border-b border-slate-800/60 align-top"
          :class="[bg(r), overId === r.id && 'tx-over', dim(r) && 'opacity-35', lit(r) && 'tx-lit']"
          @dragover="onOver($event, r)"
          @dragenter.prevent
          @dragleave="overId === r.id && (overId = null)"
          @drop="onDrop($event, r)"
        >
          <td class="px-3 py-2 whitespace-nowrap text-slate-400 text-[11px]">
            {{ dateStr(r.t.time) }}<br /><span class="text-slate-600">{{ timeStr(r.t.time) }}</span>
          </td>
          <td class="px-2 py-2">
            <template v-if="r.isCp">
              <div class="font-bold text-slate-100 break-words" :title="(r.t.description || '').trim() || undefined">{{ r.name }}</div>
              <div v-if="r.mask" class="text-[10px] text-slate-500">{{ r.mask }}</div>
            </template>
            <div v-else class="text-slate-300 break-words">{{ (r.t.counter_name || r.t.description || '—').trim() }}</div>
          </td>
          <td class="px-2 py-2">
            <div v-if="r.chips.length" class="flex flex-wrap gap-1">
              <span
                v-for="c in r.chips"
                :key="c.key"
                class="text-[9px] px-1 py-px rounded border"
                :class="c.saved ? 'border-emerald-500/50 text-emerald-300' : 'border-dashed border-slate-600 text-slate-400'"
              >{{ c.label }}</span>
            </div>
          </td>
          <td class="px-3 py-2 text-right whitespace-nowrap font-bold" :class="tone(r.t.amount)">{{ money(r.t) }}</td>
        </tr>
        <tr v-if="rows.length === 0">
          <td colspan="4" class="p-10 text-center text-slate-600 text-xs uppercase">{{ empty }}</td>
        </tr>
        <tr v-if="rows.length > visible">
          <td colspan="4" class="p-3 text-center">
            <button class="ui-btn ui-btn--sm ui-btn--outline" @click="visible += PAGE">Показать ещё ({{ rows.length - visible }})</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.tx-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgb(52 211 153 / 0.5) transparent;
}
.tx-row { transition: background-color 0.15s, opacity 0.15s, box-shadow 0.15s; }
.tx-yellow { background: rgb(245 158 11 / 0.10); box-shadow: inset 3px 0 0 rgb(245 158 11 / 0.7); }
.tx-green  { background: rgb(16 185 129 / 0.10); box-shadow: inset 3px 0 0 rgb(16 185 129 / 0.7); }
.tx-lit { box-shadow: inset 3px 0 0 rgb(52 211 153), 0 0 0 1px rgb(52 211 153 / 0.5); }
.tx-over { background: rgb(52 211 153 / 0.22) !important; box-shadow: inset 3px 0 0 rgb(52 211 153), 0 0 0 2px rgb(52 211 153) !important; }
</style>

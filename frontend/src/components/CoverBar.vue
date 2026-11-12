<script setup>
import { computed } from 'vue'
import { formatRuDate } from '../utils/formatters.js'

// Таймлайн покрытия источников счёта (та же раскраска, что в панели синка):
//   зелёный       — данные есть
//   косые полосы  — период снят, операций в нём не было
//   без заливки   — данных нет (до начала файла, после его конца, дыры между файлами)
const props = defineProps({
  cells: { type: String, default: '' },
  from: { type: Number, default: 0 },
  to: { type: Number, default: 0 }
})

const runs = computed(() => {
  const c = props.cells
  const out = []
  let i = 0
  while (i < c.length) {
    let j = i
    while (j < c.length && c[j] === c[i]) j++
    if (c[i] !== '0') out.push({ kind: c[i], left: (i / c.length) * 100, width: ((j - i) / c.length) * 100 })
    i = j
  }
  return out
})
</script>

<template>
  <div class="space-y-0.5">
    <div class="ui-tl">
      <span
        v-for="(r, i) in runs"
        :key="i"
        class="ui-tl__run"
        :class="r.kind === '2' ? 'ui-tl__run--empty' : 'ui-tl__run--data'"
        :style="{ left: r.left + '%', width: r.width + '%' }"
      ></span>
    </div>
    <div class="flex items-center justify-between text-[9px] text-slate-500">
      <span>{{ formatRuDate(from) }}</span>
      <span>{{ formatRuDate(to) }}</span>
    </div>
  </div>
</template>

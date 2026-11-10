<script setup>
import { computed } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import { formatRuDate, formatRuRange, plural } from '../utils/formatters.js'

// Прогресс-бар счёта = таймлайн: ось от открытия до «сейчас».
//   зелёный        — снято, операции есть
//   косые полосы   — снято, за период операций не было
//   жёлтые маячки  — окно, которое сейчас ждёт загрузки (красные — окно, на котором случилась ошибка)
const props = defineProps({
  acc: { type: Object, required: true },
  now: { type: Number, default: () => Date.now() }
})

const span = computed(() => Math.max(1, props.acc.axis_to - props.acc.axis_from))
const pos = (t) => Math.min(100, Math.max(0, ((t - props.acc.axis_from) / span.value) * 100))

// склеиваем соседние одинаковые клетки в отрезки
const runs = computed(() => {
  const cells = props.acc.cells || ''
  const out = []
  let i = 0
  while (i < cells.length) {
    let j = i
    while (j < cells.length && cells[j] === cells[i]) j++
    if (cells[i] !== '0') out.push({ kind: cells[i], left: (i / cells.length) * 100, width: ((j - i) / cells.length) * 100 })
    i = j
  }
  return out
})

const isError = computed(() => props.acc.state === 'error')
const isDone = computed(() => props.acc.state === 'done')
const win = computed(() => props.acc.window || null)
const showWin = computed(() => !!win.value && (isError.value || props.acc.state === 'cooling' || props.acc.state === 'fetch'))
const winLeft = computed(() => (win.value ? pos(win.value.from) : 0))
const winRight = computed(() => (win.value ? pos(win.value.to) : 0))
const narrow = computed(() => winRight.value - winLeft.value < 4) // узкое окно: стрелки смотрят наружу, чтобы не слипались
const cooling = computed(() => props.acc.state === 'cooling')
const left = computed(() => Math.max(0, props.acc.cool_until - Math.floor(props.now / 1000)))

const caption = computed(() => {
  const a = props.acc
  const range = win.value ? formatRuRange(win.value.from, win.value.to) : ''
  if (a.state === 'error') return `Ошибка: ${a.error || 'неизвестно'}${range ? ' · ' + range : ''}`
  if (a.state === 'done') return 'Готово'
  if (a.state === 'queued') return 'В очереди'
  if (!win.value) return ''
  return cooling.value
    ? `Охлаждение ${left.value} ${plural(left.value, 'секунда', 'секунды', 'секунд')} · ${range}`
    : `Запрос · ${range}`
})
</script>

<template>
  <div class="space-y-1">
    <div class="flex items-center gap-2">
      <div class="ui-tl" :class="isError && 'ui-tl--err'">
        <span
          v-for="(r, i) in runs"
          :key="i"
          class="ui-tl__run"
          :class="[r.kind === '2' ? 'ui-tl__run--empty' : 'ui-tl__run--data', isError && 'ui-tl__run--err']"
          :style="{ left: r.left + '%', width: r.width + '%' }"
        ></span>

        <template v-if="showWin">
          <span class="ui-tl__win" :class="[cooling && 'ui-tl__win--wait', isError && 'ui-tl__win--err']" :style="{ left: winLeft + '%', width: Math.max(0.5, winRight - winLeft) + '%' }"></span>
          <span class="ui-tl__pin ui-tl__pin--l" :class="[narrow && 'ui-tl__pin--out', isError && 'ui-tl__pin--err']" :style="{ left: winLeft + '%' }"></span>
          <span class="ui-tl__pin ui-tl__pin--r" :class="[narrow && 'ui-tl__pin--out', isError && 'ui-tl__pin--err']" :style="{ left: winRight + '%' }"></span>
        </template>
      </div>

      <span class="w-9 flex justify-end text-[10px] font-bold tabular-nums" :class="isError ? 'text-rose-400' : 'text-slate-300'">
        <UiIcon v-if="isDone" name="check" size="sm" class="text-emerald-400" />
        <template v-else>{{ acc.percent }}%</template>
      </span>
    </div>

    <div class="flex items-center justify-between gap-2 text-[9px] text-slate-500 pr-11">
      <span>{{ formatRuDate(acc.axis_from) }}</span>
      <span>{{ formatRuDate(acc.axis_to) }}</span>
    </div>
    <div v-if="caption" class="text-[9px] pr-11" :class="isError ? 'text-rose-400 break-words' : 'whitespace-nowrap overflow-hidden text-ellipsis ' + ( (cooling ? 'text-amber-400' : isDone ? 'text-emerald-400' : 'text-slate-400'))">{{ caption }}</div>
  </div>
</template>

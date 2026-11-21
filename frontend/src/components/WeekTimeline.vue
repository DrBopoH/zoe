<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import { formatMoney } from '../utils/formatters.js'
import { timelineSummary, addDays } from '../utils/glovoLedger.js'

// Таймлайн номинального дохода: неделя за неделей, листается вбок. На каждой неделе — номинал, который записал Glovo (вносится прямо в карточке),
// и сколько реально пришло на карту в следующую неделю (выплата приходит через 2–4 дня после отчёта). Разница — удержанные налоги и вычеты.
const props = defineProps({
  rows: { type: Array, default: () => [] }, // результат timelineWeeks()
  currentWeek: { type: String, default: '' } // понедельник текущей недели
})
const emit = defineEmits(['save', 'remove', 'older'])

const summary = computed(() => timelineSummary(props.rows))
const fmtDay = (iso) => `${iso.slice(8, 10)}.${iso.slice(5, 7)}`
const range = (from) => `${fmtDay(from)} — ${fmtDay(addDays(from, 6))}`
const arrivalRange = (r) => `${fmtDay(addDays(r.week, 7))} — ${fmtDay(addDays(r.week, 13))}`
const money = (v) => formatMoney(v)

// ── прокрутка ──
const scroller = ref(null)
const toEnd = () => { if (scroller.value) scroller.value.scrollLeft = scroller.value.scrollWidth }
const by = (dir) => scroller.value?.scrollBy({ left: dir * 3 * 172, behavior: 'smooth' })
onMounted(() => nextTick(toEnd))
// первое появление строк (данные приходят позже монтирования) — тоже к текущей неделе
let positioned = false
watch(() => props.rows.length, (n) => { if (n && !positioned) { positioned = true; nextTick(toEnd) } }, { immediate: true })

// ── правка номинала прямо в карточке ──
const editing = ref('') // понедельник недели в правке
const draft = ref('')
const bad = ref(false)
const input = ref(null)
const startEdit = (r) => {
  editing.value = r.week
  draft.value = r.nominal === null ? '' : String(r.nominal).replace('.', ',')
  bad.value = false
  nextTick(() => { const el = Array.isArray(input.value) ? input.value[0] : input.value; el?.focus(); el?.select() })
}
const cancel = () => { editing.value = ''; bad.value = false }
const commit = (r) => {
  if (editing.value !== r.week) return
  const text = draft.value.trim()
  if (text === '') {
    if (r.nominal !== null) emit('remove', r.week)
    cancel()
    return
  }
  const n = Number(text.replace(/\s/g, '').replace(',', '.'))
  if (!Number.isFinite(n) || n < 0) { bad.value = true; return }
  if (n !== r.nominal) emit('save', r.week, n)
  cancel()
}

const STATUS = {
  paid: 'border-emerald-500/30 bg-emerald-500/[0.04]',
  pending: 'border-amber-500/30 border-dashed',
  missing: 'border-rose-500/40',
  none: 'border-slate-800'
}
</script>

<template>
  <div class="space-y-3" data-testid="timeline">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-[11px] text-slate-500" data-testid="timeline-summary">
        <span>Номинал <b class="text-slate-200 tabular-nums">{{ money(summary.nominal) }}</b></span>
        <span>Пришло <b class="text-emerald-400 tabular-nums">{{ money(summary.received) }}</b></span>
        <span>Удержано <b class="text-rose-400 tabular-nums">{{ money(summary.withheld) }}</b><template v-if="summary.withheldPct !== null"> ({{ String(summary.withheldPct).replace('.', ',') }} %)</template></span>
        <span class="text-slate-600">по {{ summary.weeks }} нед. с пришедшей выплатой</span>
      </div>
      <div class="flex items-center gap-1">
        <button type="button" class="tl-nav" aria-label="Назад по неделям" title="Раньше" @click="by(-1)"><span class="inline-flex rotate-180"><UiIcon name="chevron" /></span></button>
        <button type="button" class="tl-nav tl-nav--text" @click="toEnd">Сегодня</button>
        <button type="button" class="tl-nav" aria-label="Вперёд по неделям" title="Позже" @click="by(1)"><UiIcon name="chevron" /></button>
      </div>
    </div>

    <div v-if="!rows.length" class="text-xs text-slate-500 px-1 py-4">Недель пока нет.</div>

    <div v-else ref="scroller" class="tl-scroll" role="list" data-testid="timeline-scroll">
      <button type="button" class="tl-older" data-testid="timeline-older" @click="emit('older')">
        <span class="inline-flex rotate-180"><UiIcon name="chevron" /></span>
        <span>Раньше</span>
      </button>
      <article
        v-for="r in rows"
        :key="r.week"
        role="listitem"
        class="tl-card"
        :class="[STATUS[r.status], r.week === currentWeek && 'tl-card--now']"
        :data-week="r.week"
      >
        <header class="flex items-center justify-between gap-1">
          <span class="text-[11px] font-bold text-slate-300 tabular-nums whitespace-nowrap">{{ range(r.week) }}</span>
          <span v-if="r.week === currentWeek" class="text-[9px] font-bold uppercase tracking-wider text-emerald-400 whitespace-nowrap">эта неделя</span>
        </header>

        <div class="space-y-0.5">
          <div class="text-[9px] uppercase tracking-wider text-slate-500">Номинал</div>
          <template v-if="editing === r.week">
            <input
              ref="input"
              v-model="draft"
              type="text"
              inputmode="decimal"
              class="ui-input !py-1 !text-sm"
              :class="bad && '!border-rose-500'"
              placeholder="Сумма, ₴"
              aria-label="Номинальный доход за неделю"
              @keydown.enter.prevent="commit(r)"
              @keydown.esc.prevent="cancel"
              @blur="commit(r)"
            />
            <div class="text-[9px] text-slate-600">Enter — записать, Esc — отмена, пусто — удалить</div>
          </template>
          <button
            v-else-if="r.nominal !== null"
            type="button"
            class="tl-value"
            title="Изменить номинал"
            @click="startEdit(r)"
          >{{ money(r.nominal) }}</button>
          <button v-else type="button" class="tl-add" @click="startEdit(r)">+ Внести</button>
        </div>

        <div class="space-y-0.5">
          <div class="text-[9px] uppercase tracking-wider text-slate-500" :title="`Выплата приходит на следующей неделе: ${arrivalRange(r)}`">Пришло {{ arrivalRange(r) }}</div>
          <div class="text-sm font-bold tabular-nums" :class="r.count ? 'text-emerald-400' : 'text-slate-600'">{{ r.count ? money(r.received) : '—' }}</div>
          <div v-if="r.count" class="text-[10px] text-slate-500">первая выплата {{ fmtDay(r.firstDate) }}, через {{ r.lagDays }} дн.<template v-if="r.count > 1"> · всего {{ r.count }}</template></div>
          <div v-else-if="r.status === 'pending'" class="text-[10px] text-amber-400/80">ждём до {{ fmtDay(addDays(r.week, 13)) }}</div>
          <div v-else-if="r.status === 'missing'" class="text-[10px] text-rose-400">выплата не пришла</div>
        </div>

        <footer v-if="r.delta !== null" class="text-[10px] tabular-nums" :class="r.delta <= 0 ? 'text-rose-300' : 'text-emerald-300'">
          <template v-if="r.delta <= 0">Удержано {{ money(-r.delta) }}<template v-if="r.deltaPct !== null"> ({{ String(r.deltaPct).replace('.', ',') }} %)</template></template>
          <template v-else>Больше номинала на {{ money(r.delta) }}</template>
        </footer>
      </article>
    </div>
  </div>
</template>

<style scoped>
.tl-scroll {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
  scroll-snap-type: x proximity;
  padding: 0.25rem 0.25rem 0.75rem;
  /* затухание краёв: видно, что лента продолжается */
  mask-image: linear-gradient(to right, transparent 0, #000 1.25rem, #000 calc(100% - 1.25rem), transparent 100%);
  scrollbar-width: thin;
  scrollbar-color: rgb(51 65 85) transparent;
}
.tl-card {
  flex: 0 0 10.5rem;
  scroll-snap-align: end;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.65rem 0.75rem;
  border-width: 1px;
  border-radius: 0.5rem;
  background: rgb(15 23 42 / 0.6);
  transition: border-color 0.15s;
}
.tl-card--now { box-shadow: 0 0 0 1px rgb(52 211 153 / 0.55); }
.tl-value {
  display: block;
  width: 100%;
  text-align: left;
  font-size: 0.95rem;
  font-weight: 700;
  color: rgb(226 232 240);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  border-radius: 0.25rem;
  transition: color 0.15s;
}
.tl-value:hover { color: rgb(110 231 183); }
.tl-add {
  width: 100%;
  padding: 0.3rem 0;
  border: 1px dashed rgb(51 65 85);
  border-radius: 0.375rem;
  font-size: 0.7rem;
  font-weight: 700;
  color: rgb(100 116 139);
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}
.tl-add:hover { color: rgb(110 231 183); border-color: rgb(16 185 129 / 0.5); }
.tl-older {
  flex: 0 0 4.5rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  border: 1px dashed rgb(51 65 85);
  border-radius: 0.5rem;
  font-size: 0.65rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: rgb(100 116 139);
  cursor: pointer;
  scroll-snap-align: start;
  transition: color 0.15s, border-color 0.15s;
}
.tl-older:hover { color: rgb(110 231 183); border-color: rgb(16 185 129 / 0.5); }
.tl-nav {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 1.75rem;
  min-width: 1.75rem;
  padding: 0 0.4rem;
  border: 1px solid rgb(30 41 59);
  border-radius: 0.375rem;
  background: rgb(15 23 42 / 0.6);
  color: rgb(148 163 184);
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}
.tl-nav:hover { color: rgb(110 231 183); border-color: rgb(16 185 129 / 0.4); }
.tl-nav--text { font-size: 0.65rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.06em; }
</style>

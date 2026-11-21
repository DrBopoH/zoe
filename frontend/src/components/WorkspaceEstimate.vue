<script setup>
import { computed, onMounted, watch } from 'vue'
import FlowChart from './FlowChart.vue'
import UiLoadError from './ui/UiLoadError.vue'
import UiEmpty from './ui/UiEmpty.vue'
import { estimateModel, TAX_DEFAULTS } from '../utils/glovoLedger.js'
import { pushRequestError } from '../composables/useToasts.js'
import { useWorkspaceLedger } from '../composables/useWorkspaceLedger.js'
import { formatMoney, toLocalDateStr } from '../utils/formatters.js'

// Смета дела «Glovo»: сведение учёта — сколько записал Glovo, сколько дошло до карт, сколько ушло на расходы и убытки и что осталось.
// Данные те же, что в «Учёте» (бейджи и недельный номинал), здесь только чтение.
const props = defineProps({
  workspaceId: { type: String, required: true },
  transactions: { type: Array, default: () => [] }
})
const { marks, weeks, params, error, load, setParam, resetParam } = useWorkspaceLedger(() => props.workspaceId)
onMounted(load)
watch(() => props.workspaceId, load)

const m = computed(() => estimateModel({ transactions: props.transactions, marks: marks.value, weeks: weeks.value, params: params.value, today: toLocalDateStr(new Date()) }))
const empty = computed(() => !m.value.flow.length && !m.value.nominal)
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')

// Сведение: от номинала до «в кармане». Полоса — доля от самой большой суммы.
const steps = computed(() => {
  const x = m.value
  const rows = [
    { key: 'nominal', label: 'Номинал Glovo', hint: `${x.weeks} нед. с выплатой`, v: x.nominal, color: '#94a3b8' },
    { key: 'withheld', label: x.withheld < 0 ? 'Сверх номинала' : 'Удержано', hint: x.withheldPct === null ? '' : `${Math.abs(x.withheldPct)}% номинала`, v: -x.withheld, color: '#f43f5e' },
    { key: 'taxable', label: 'Пришло на карты', hint: 'Доход + Выплата', v: x.taxable, color: '#34d399' },
    { key: 'tips', label: 'Чай', hint: 'наличные', v: x.cash, color: '#f59e0b' },
    { key: 'out', label: 'Расходы и убытки', hint: `Расход ${formatMoney(x.totals.expense.sum)} · Убыток ${formatMoney(x.totals.loss.sum)}`, v: x.outflow, color: '#f97316' },
    ...(x.taxes.months.length ? [{ key: 'tax', label: 'Налоги ФОП', hint: `ЕП ${formatMoney(x.taxes.tax)} · ВС ${formatMoney(x.taxes.war)} · ЕСВ ${formatMoney(x.taxes.esv)}`, v: -x.taxes.total, color: '#a78bfa' }] : []),
    { key: 'net', label: 'В кармане', hint: x.taxes.months.length ? 'приход − уход − налоги' : 'приход минус уход', v: x.afterTax, color: '#38bdf8', strong: true }
  ]
  const max = Math.max(1, ...rows.map((r) => Math.abs(r.v)))
  return rows.map((r) => ({ ...r, pct: Math.min(100, (Math.abs(r.v) / max) * 100) }))
})

// Параметры налога ФОП: пусто или как по умолчанию — значение сбрасывается на умолчание, иначе сохраняется своё
const TAX_FIELDS = [
  { key: 'tax_rate', label: 'Единый налог, %', step: '0.1' },
  { key: 'war_rate', label: 'Военный сбор, %', step: '0.1' },
  { key: 'esv', label: 'ЕСВ в месяц, ₴', step: '0.01' }
]
const paramValue = (k) => (params.value.has(k) ? params.value.get(k) : TAX_DEFAULTS[k])
const onParam = async (k, ev) => {
  const raw = String(ev.target.value).trim().replace(',', '.')
  const n = Number(raw)
  try {
    if (raw === '' || (Number.isFinite(n) && n === TAX_DEFAULTS[k])) await resetParam(k)
    else if (Number.isFinite(n) && n >= 0) await setParam(k, n)
    else ev.target.value = paramValue(k)
  } catch (e) {
    pushRequestError('Не удалось сохранить параметр налога', e)
    ev.target.value = paramValue(k)
  }
}
const monthLabel = (m) => `${m.slice(5, 7)}.${m.slice(0, 4)}`
</script>

<template>
  <div class="space-y-6 font-mono select-none">
    <UiLoadError v-if="error" :message="error" @retry="load" />
    <UiEmpty v-if="empty">Смета заполнится сама: отметь счета и поставь бейджи в «Учёте», внеси номинал по неделям</UiEmpty>
    <template v-else>
      <section class="space-y-3">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Сведение</div>
        <div class="space-y-2" data-testid="estimate-steps">
          <div v-for="s in steps" :key="s.key" class="grid grid-cols-[11rem_1fr_9rem] items-center gap-3" :data-step="s.key">
            <div>
              <div class="text-[11px] font-bold" :class="s.strong ? 'text-slate-100' : 'text-slate-300'">{{ s.label }}</div>
              <div class="text-[10px] text-slate-500 truncate">{{ s.hint }}</div>
            </div>
            <div class="h-2 rounded bg-slate-800 overflow-hidden"><div class="h-full rounded" :style="{ width: s.pct + '%', background: s.color }" /></div>
            <div class="text-right text-sm font-bold tabular-nums" :class="s.strong ? tone(s.v) : 'text-slate-200'">{{ formatMoney(s.v) }}</div>
          </div>
        </div>
      </section>

      <section class="space-y-3" data-testid="tax-block">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Налоги ФОП</div>
        <p class="text-[11px] text-slate-500">Единый налог и военный сбор считаются с «Выплат» (на счёт ФОП приходит вся сумма), ЕСВ фиксированный за каждый месяц с первой выплаты. «Доход» на сберкарту Glovo облагает сам, чай наличный. Значения по умолчанию — ФОП 3 группы 2026, любое можно поменять.</p>
        <div class="flex flex-wrap gap-4">
          <label v-for="f in TAX_FIELDS" :key="f.key" class="space-y-1">
            <span class="block text-[10px] uppercase tracking-wider font-bold text-slate-500">{{ f.label }}</span>
            <input class="ui-input w-36 tabular-nums" type="number" min="0" :step="f.step" :value="paramValue(f.key)" :data-param="f.key" @change="onParam(f.key, $event)" />
          </label>
        </div>
        <div v-if="m.taxes.months.length" class="overflow-x-auto border border-slate-800 rounded bg-slate-900/60">
          <table class="w-full text-xs tabular-nums" data-testid="tax-months">
            <thead><tr class="text-[10px] uppercase tracking-wider text-slate-500"><th class="text-left px-3 py-2">Месяц</th><th class="text-right px-3 py-2">Выплаты</th><th class="text-right px-3 py-2">Единый налог</th><th class="text-right px-3 py-2">Военный сбор</th><th class="text-right px-3 py-2">ЕСВ</th><th class="text-right px-3 py-2">Итого</th></tr></thead>
            <tbody>
              <tr v-for="r in m.taxes.months" :key="r.month" class="border-t border-slate-800" :data-month="r.month">
                <td class="px-3 py-2 text-slate-300">{{ monthLabel(r.month) }}</td>
                <td class="px-3 py-2 text-right text-sky-300">{{ formatMoney(r.payout) }}</td>
                <td class="px-3 py-2 text-right">{{ formatMoney(r.tax) }}</td>
                <td class="px-3 py-2 text-right">{{ formatMoney(r.war) }}</td>
                <td class="px-3 py-2 text-right">{{ formatMoney(r.esv) }}</td>
                <td class="px-3 py-2 text-right font-bold text-violet-300">{{ formatMoney(r.total) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="text-[11px] text-slate-500">Налогов пока нет: они появятся с первой операцией, помеченной «Выплата».</p>
      </section>

      <section class="space-y-3">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Деньги по неделям</div>
        <p class="text-[11px] text-slate-500">Неделя — по дате операции. Вверх приход, вниз расходы и убытки, белая линия — накопленный итог.</p>
        <FlowChart :flow="m.flow" />
      </section>
    </template>
  </div>
</template>

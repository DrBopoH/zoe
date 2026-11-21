<script setup>
import { computed, onMounted, watch } from 'vue'
import FlowChart from './FlowChart.vue'
import WaterfallChart from './WaterfallChart.vue'
import UiLoadError from './ui/UiLoadError.vue'
import UiEmpty from './ui/UiEmpty.vue'
import { estimateModel, withCash, TAX_DEFAULTS } from '../utils/glovoLedger.js'
import { pushRequestError } from '../composables/useToasts.js'
import { useWorkspaceLedger } from '../composables/useWorkspaceLedger.js'
import { formatMoney, toLocalDateStr } from '../utils/formatters.js'

// Смета дела «Glovo»: сведение учёта — сколько записал Glovo, сколько дошло до карт, сколько ушло на расходы и убытки и что осталось.
// Данные те же, что в «Учёте» (бейджи и недельный номинал), здесь только чтение.
const props = defineProps({
  workspaceId: { type: String, required: true },
  transactions: { type: Array, default: () => [] }
})
const { marks, weeks, params, entries, error, load, setParam, resetParam } = useWorkspaceLedger(() => props.workspaceId)
onMounted(load)
watch(() => props.workspaceId, load)

const merged = computed(() => withCash(props.transactions, marks.value, entries.value))
const m = computed(() => estimateModel({ transactions: merged.value.transactions, marks: merged.value.marks, weeks: weeks.value, params: params.value, today: toLocalDateStr(new Date()) }))
const empty = computed(() => !m.value.flow.length && !m.value.nominal)
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')

// Карточки и каскад строятся из одной модели. data-step — ключи для тестов и подсказок.
const hasTax = computed(() => m.value.taxes.months.length > 0)
const kpis = computed(() => {
  const x = m.value
  return [
    { key: 'taxable', label: 'Пришло на карты', hint: 'Доход + Выплата', v: x.taxable, dot: '#34d399' },
    { key: 'tips', label: 'Чай', hint: 'наличные', v: x.cash, dot: '#f59e0b' },
    { key: 'out', label: 'Расходы и убытки', hint: `Расход ${formatMoney(x.totals.expense.sum)} · Убыток ${formatMoney(x.totals.loss.sum)}`, v: x.outflow, dot: '#f97316' },
    ...(hasTax.value ? [{ key: 'tax', label: 'Налоги ФОП', hint: `ЕП ${formatMoney(x.taxes.tax)} · ВС ${formatMoney(x.taxes.war)} · ЕСВ ${formatMoney(x.taxes.esv)}`, v: -x.taxes.total, dot: '#a78bfa' }] : [])
  ]
})
const waterfall = computed(() => {
  const x = m.value
  const t = x.totals
  return [
    { key: 'taxable', label: 'Приход', v: x.taxable, color: '#34d399' },
    { key: 'tips', label: 'Чай', v: x.cash, color: '#f59e0b' },
    { key: 'expense', label: 'Расход', v: t.expense.sum, color: '#f97316' },
    { key: 'loss', label: 'Убыток', v: t.loss.sum, color: '#f43f5e' },
    ...(hasTax.value ? [
      { key: 'ep', label: 'Ед. налог', v: -x.taxes.tax, color: '#a78bfa' },
      { key: 'vz', label: 'Воен. сбор', v: -x.taxes.war, color: '#8b5cf6' },
      { key: 'esv', label: 'ЕСВ', v: -x.taxes.esv, color: '#7c3aed' }
    ] : []),
    { key: 'net', label: 'В кармане', v: x.afterTax, color: '#38bdf8', total: true }
  ]
})
// Сверка с Glovo: сколько записано номиналом и сколько дошло (только недели, по которым выплата уже пришла)
const recon = computed(() => {
  const x = m.value
  const pct = x.nominal > 0 ? Math.min(100, (x.received / x.nominal) * 100) : 0
  return { pct, over: x.withheld < 0, label: x.withheld < 0 ? 'Сверх номинала' : 'Удержано', amount: Math.abs(x.withheld), share: x.withheldPct === null ? '' : `${Math.abs(x.withheldPct)}%` }
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
  <div class="space-y-5 font-mono select-none">
    <UiLoadError v-if="error" :message="error" @retry="load" />
    <UiEmpty v-if="empty">Смета заполнится сама: отметь счета и поставь бейджи в «Учёте», внеси номинал по неделям</UiEmpty>
    <template v-else>
      <!-- Итог и четыре причины, из чего он сложился -->
      <section class="grid grid-cols-2 lg:grid-cols-6 gap-3" data-testid="estimate-kpis">
        <div class="col-span-2 rounded-lg border border-sky-500/30 bg-gradient-to-br from-sky-500/10 to-slate-900/60 p-4 flex flex-col justify-between" data-step="net">
          <div class="text-[10px] uppercase tracking-wider font-bold text-sky-300">В кармане</div>
          <div class="text-3xl font-bold tabular-nums my-1" :class="tone(m.afterTax)">{{ formatMoney(m.afterTax) }}</div>
          <div class="text-[10px] text-slate-500">{{ hasTax ? 'приход − уход − налоги' : 'приход минус уход' }}</div>
        </div>
        <div v-for="k in kpis" :key="k.key" class="rounded-lg border border-slate-800 bg-slate-900/60 p-3 min-w-0" :data-step="k.key">
          <div class="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold text-slate-400"><i class="w-1.5 h-1.5 rounded-full inline-block" :style="{ background: k.dot }" />{{ k.label }}</div>
          <div class="text-lg font-bold tabular-nums text-slate-100 mt-1">{{ formatMoney(k.v) }}</div>
          <div class="text-[10px] text-slate-500 truncate" :title="k.hint">{{ k.hint }}</div>
        </div>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">От прихода к «в кармане»</div>
        <WaterfallChart :steps="waterfall" />
      </section>

      <!-- Сверка номинала с тем, что реально пришло -->
      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="reconciliation">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Сверка с Glovo</div>
          <div class="text-[10px] text-slate-500">{{ m.weeks }} нед. с пришедшей выплатой</div>
        </div>
        <div class="h-2 rounded bg-slate-800 overflow-hidden"><div class="h-full rounded" :class="recon.over ? 'bg-emerald-400' : 'bg-sky-400'" :style="{ width: recon.pct + '%' }" /></div>
        <div class="grid grid-cols-3 gap-3 text-xs">
          <div data-step="nominal"><div class="text-[10px] uppercase tracking-wider text-slate-500">Номинал</div><div class="font-bold tabular-nums text-slate-200">{{ formatMoney(m.nominal) }}</div></div>
          <div data-step="received"><div class="text-[10px] uppercase tracking-wider text-slate-500">Пришло</div><div class="font-bold tabular-nums text-slate-200">{{ formatMoney(m.received) }}</div></div>
          <div data-step="withheld"><div class="text-[10px] uppercase tracking-wider text-slate-500">{{ recon.label }}</div><div class="font-bold tabular-nums" :class="recon.over ? 'text-emerald-400' : 'text-rose-400'">{{ formatMoney(recon.amount) }} <span v-if="recon.share" class="text-[10px] text-slate-500">{{ recon.share }}</span></div></div>
        </div>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="tax-block">
        <div class="flex flex-wrap items-end justify-between gap-3">
          <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Налоги ФОП</div>
          <div class="flex flex-wrap gap-3">
            <label v-for="f in TAX_FIELDS" :key="f.key" class="flex items-center gap-1.5">
              <span class="text-[10px] uppercase tracking-wider font-bold text-slate-500">{{ f.label }}</span>
              <input class="ui-input w-24 tabular-nums !py-1" type="number" min="0" :step="f.step" :value="paramValue(f.key)" :data-param="f.key" @change="onParam(f.key, $event)" />
            </label>
          </div>
        </div>
        <p class="text-[11px] text-slate-500">С «Выплат» (на счёт ФОП приходит вся сумма) считаются единый налог и военный сбор, ЕСВ фиксированный за каждый месяц с первой выплаты. «Доход» на сберкарту Glovo облагает сам, чай наличный. По умолчанию ФОП 3 группы 2026.</p>
        <div v-if="hasTax" class="overflow-x-auto">
          <table class="w-full text-xs tabular-nums" data-testid="tax-months">
            <thead><tr class="text-[10px] uppercase tracking-wider text-slate-500"><th class="text-left py-2">Месяц</th><th class="text-right py-2">Выплаты</th><th class="text-right py-2">Ед. налог</th><th class="text-right py-2">Воен. сбор</th><th class="text-right py-2">ЕСВ</th><th class="text-right py-2">Итого</th></tr></thead>
            <tbody>
              <tr v-for="r in m.taxes.months" :key="r.month" class="border-t border-slate-800" :data-month="r.month">
                <td class="py-2 text-slate-300">{{ monthLabel(r.month) }}</td>
                <td class="py-2 text-right text-sky-300">{{ formatMoney(r.payout) }}</td>
                <td class="py-2 text-right">{{ formatMoney(r.tax) }}</td>
                <td class="py-2 text-right">{{ formatMoney(r.war) }}</td>
                <td class="py-2 text-right">{{ formatMoney(r.esv) }}</td>
                <td class="py-2 text-right font-bold text-violet-300">{{ formatMoney(r.total) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="text-[11px] text-slate-500">Налогов пока нет: они появятся с первой операцией, помеченной «Выплата».</p>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Деньги по неделям</div>
        <p class="text-[11px] text-slate-500">Неделя — по дате операции. Вверх приход, вниз расходы и убытки, белая линия — накопленный итог.</p>
        <FlowChart :flow="m.flow" />
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, watch } from 'vue'
import UiLoadError from './ui/UiLoadError.vue'
import UiEmpty from './ui/UiEmpty.vue'
import { useWorkspaceLedger } from '../composables/useWorkspaceLedger.js'
import { labEstimate, LAB_BADGES } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'
import { dmy } from '../utils/numfmt.js'

// Смета ХимЛаба: деньги (Продажа, Расход, Убыток), куда ушло сырьё, партии с себестоимостью, продажами и сжиганием, итог с запасами.
// Документы (карточки, партии, движения, списания) приходят со страницы, бейджи и наличные записи читаются здесь. Только чтение.
const props = defineProps({
  workspaceId: { type: String, required: true },
  transactions: { type: Array, default: () => [] },
  cards: { type: Array, default: () => [] },
  batches: { type: Array, default: () => [] },
  moves: { type: Array, default: () => [] },
  writeoffs: { type: Array, default: () => [] }
})
const { marks, entries, error, load } = useWorkspaceLedger(() => props.workspaceId)
onMounted(load)
watch(() => props.workspaceId, load)

const m = computed(() => labEstimate({ transactions: props.transactions, marks: marks.value, entries: entries.value, cards: props.cards, batches: props.batches, moves: props.moves, writeoffs: props.writeoffs }))
const empty = computed(() => !props.cards.length && !props.batches.length && !m.value.bank.sale.count && !m.value.bank.expense.count && !m.value.bank.loss.count)
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')
const label = (k) => LAB_BADGES.find((b) => b.key === k)

const kpis = computed(() => [
  { key: 'raw', label: 'Сырьё на складе', hint: `закуплено ${formatMoney(m.value.stock.purchased)}`, v: m.value.stock.left, dot: '#34d399' },
  { key: 'finished', label: 'Готовая продукция', hint: 'остаток штук по себестоимости', v: m.value.finished, dot: '#38bdf8' },
  { key: 'comp', label: 'Компенсации', hint: `сожжено ${m.value.burns.pieces} шт по себестоимости`, v: m.value.burns.compensation, dot: '#f97316' },
  { key: 'equity', label: 'Итог с запасами', hint: 'деньги + запасы + компенсации', v: m.value.equity, dot: '#a78bfa' }
])
// из чего сложилась закупка: осталось, ушло в партии, списано
const split = computed(() => {
  const s = m.value.stock
  const total = s.purchased || 1
  return [
    { key: 'left', label: 'Осталось на складе', v: s.left, cls: 'bg-emerald-400/80' },
    { key: 'batches', label: 'Ушло в партии', v: s.inBatches, cls: 'bg-sky-400/80' },
    { key: 'written', label: 'Списано в убыток', v: s.writtenOff, cls: 'bg-rose-400/80' }
  ].map((x) => ({ ...x, pct: Math.max(0, Math.min(100, (x.v / total) * 100)) }))
})
</script>

<template>
  <div class="space-y-5 font-mono select-none" data-testid="lab-estimate">
    <UiLoadError v-if="error" :message="error" @retry="load" />
    <UiEmpty v-if="empty">Смета заполнится сама: внеси карточки товаров в «Учёте», сделай партии и поставь бейджи «Продажа», «Расход», «Убыток»</UiEmpty>
    <template v-else>
      <section class="grid grid-cols-2 lg:grid-cols-6 gap-3" data-testid="estimate-kpis">
        <div class="col-span-2 rounded-lg border border-sky-500/30 bg-gradient-to-br from-sky-500/10 to-slate-900/60 p-4 flex flex-col justify-between" data-step="cash">
          <div class="text-[10px] uppercase tracking-wider font-bold text-sky-300">Деньги лаба</div>
          <div class="text-3xl font-bold tabular-nums my-1" :class="tone(m.cash)">{{ formatMoney(m.cash) }}</div>
          <div class="text-[10px] text-slate-500">продажи минус расходы и убытки</div>
        </div>
        <div v-for="k in kpis" :key="k.key" class="rounded-lg border border-slate-800 bg-slate-900/60 p-3 min-w-0" :data-step="k.key">
          <div class="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold text-slate-400"><i class="w-1.5 h-1.5 rounded-full inline-block" :style="{ background: k.dot }" />{{ k.label }}</div>
          <div class="text-lg font-bold tabular-nums mt-1" :class="k.key === 'equity' ? tone(k.v) : 'text-slate-100'">{{ formatMoney(k.v) }}</div>
          <div class="text-[10px] text-slate-500 truncate" :title="k.hint">{{ k.hint }}</div>
        </div>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="estimate-money">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Деньги</div>
        <div class="grid grid-cols-3 gap-3 text-xs">
          <div v-for="k in ['sale', 'expense', 'loss']" :key="k" :data-step="k">
            <div class="text-[10px] uppercase tracking-wider font-bold" :class="label(k).cls.split(' ').filter((x) => x.startsWith('text')).join(' ')">{{ label(k).label }}</div>
            <div class="font-bold tabular-nums" :class="tone(m.bank[k].sum)">{{ formatMoney(m.bank[k].sum) }}</div>
            <div class="text-[10px] text-slate-500">{{ m.bank[k].count }} опер.</div>
          </div>
        </div>
        <p class="text-[11px] text-slate-500">Только реальные деньги: операции с бейджами и наличные записи. Убыток от списания остатка сюда не входит, сырьё уже оплачено закупкой (оно в разборе ниже).</p>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="estimate-stock">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Куда ушло сырьё</div>
          <div class="text-[10px] text-slate-500">{{ m.stock.substances }} веществ · {{ m.stock.cards }} карточ. · закуплено {{ formatMoney(m.stock.purchased) }}</div>
        </div>
        <div class="flex h-2 rounded bg-slate-800 overflow-hidden" role="img" aria-label="Из чего сложилась закупка">
          <div v-for="x in split" :key="x.key" :class="x.cls" :style="{ width: x.pct + '%' }" />
        </div>
        <div class="grid grid-cols-3 gap-3 text-xs">
          <div v-for="x in split" :key="x.key" :data-step="'split-' + x.key">
            <div class="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-slate-500"><i class="w-1.5 h-1.5 rounded-full inline-block" :class="x.cls" />{{ x.label }}</div>
            <div class="font-bold tabular-nums text-slate-200">{{ formatMoney(x.v) }}</div>
          </div>
        </div>
        <p v-if="Math.abs(m.stock.gap) >= 0.01" class="text-[11px] text-amber-300" data-testid="stock-gap">Не сходится на {{ formatMoney(m.stock.gap) }}: вероятно, удалена карточка, на которую ссылалась партия или списание.</p>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="estimate-batches">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Партии</div>
          <div class="text-[10px] text-slate-500">{{ m.production.count }} партий · {{ m.production.pieces }} шт · себестоимость {{ formatMoney(m.production.cost) }}</div>
        </div>
        <UiEmpty v-if="!m.batches.length">Партий пока нет</UiEmpty>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-xs tabular-nums">
            <thead><tr class="text-[10px] uppercase tracking-wider text-slate-500">
              <th class="text-left py-2">Партия</th><th class="text-right py-2">Штук</th><th class="text-right py-2">Себестоимость</th><th class="text-right py-2">За штуку</th>
              <th class="text-right py-2">Продано</th><th class="text-right py-2">Наценка</th><th class="text-right py-2">Сожжено</th><th class="text-right py-2">В наличии</th>
            </tr></thead>
            <tbody>
              <tr v-for="r in m.batches" :key="r.id" class="border-t border-slate-800" :data-batch="r.code">
                <td class="py-2 text-slate-200 font-bold">{{ r.code }} <span class="text-[10px] text-slate-500 font-normal">{{ dmy(r.day) }}</span></td>
                <td class="py-2 text-right">{{ r.pieces || '—' }}</td>
                <td class="py-2 text-right">{{ formatMoney(r.cost) }}</td>
                <td class="py-2 text-right text-emerald-400">{{ r.unit === null ? '—' : formatMoney(r.unit) }}</td>
                <td class="py-2 text-right">{{ r.soldPieces }} шт · {{ formatMoney(r.revenue) }}</td>
                <td class="py-2 text-right" :class="tone(r.margin)">{{ r.soldPieces ? formatMoney(r.margin) : '—' }}</td>
                <td class="py-2 text-right">{{ r.burnedPieces }} шт · {{ formatMoney(r.compensation) }}</td>
                <td class="py-2 text-right text-sky-300">{{ r.left }} шт · {{ formatMoney(r.leftValue) }}</td>
              </tr>
            </tbody>
            <tfoot><tr class="border-t border-slate-700 font-bold text-slate-200">
              <td class="py-2">Всего</td><td class="py-2 text-right">{{ m.production.pieces }}</td><td class="py-2 text-right">{{ formatMoney(m.production.cost) }}</td><td class="py-2" />
              <td class="py-2 text-right">{{ m.sales.pieces }} шт · {{ formatMoney(m.sales.revenue) }}</td><td class="py-2 text-right" :class="tone(m.sales.margin)">{{ formatMoney(m.sales.margin) }}</td>
              <td class="py-2 text-right">{{ m.burns.pieces }} шт · {{ formatMoney(m.burns.compensation) }}</td><td class="py-2 text-right">{{ formatMoney(m.finished) }}</td>
            </tr></tfoot>
          </table>
        </div>
      </section>

      <section class="rounded-lg border border-slate-800 bg-slate-900/60 p-4 space-y-3" data-testid="estimate-recon">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Сверка продаж</div>
        <div class="grid grid-cols-3 gap-3 text-xs">
          <div data-step="sold"><div class="text-[10px] uppercase tracking-wider text-slate-500">Продано (движения)</div><div class="font-bold tabular-nums text-slate-200">{{ formatMoney(m.recon.sold) }}</div></div>
          <div data-step="received"><div class="text-[10px] uppercase tracking-wider text-slate-500">Пришло (бейдж «Продажа»)</div><div class="font-bold tabular-nums text-slate-200">{{ formatMoney(m.recon.received) }}</div></div>
          <div data-step="diff"><div class="text-[10px] uppercase tracking-wider text-slate-500">{{ m.recon.diff < 0 ? 'Ещё не пришло' : 'Пришло сверх' }}</div><div class="font-bold tabular-nums" :class="m.recon.diff < 0 ? 'text-amber-300' : 'text-emerald-400'">{{ formatMoney(Math.abs(m.recon.diff)) }}</div></div>
        </div>
        <p class="text-[11px] text-slate-500">Продажа в партиях — что и по какой цене продано, бейдж «Продажа» — деньги, которые пришли на счёт. В «Деньги лаба» входит только второе, поэтому двойного счёта нет.</p>
      </section>
    </template>
  </div>
</template>

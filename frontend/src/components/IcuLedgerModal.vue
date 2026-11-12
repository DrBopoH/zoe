<script setup>
import { ref, watch } from 'vue'
import { apiGetJson, q } from '../utils/api.js'
import { formatMoney } from '../utils/formatters.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'

// Брокерский счёт ICU: только просмотр. Всё приходит из брокерского отчёта (загружается как выписка банка,
// вкладка отладки → Импорт / Загрузить); руками здесь ничего не вводится и не правится.
const props = defineProps({
  account: { type: Object, default: null },
  isOpen: { type: Boolean, default: false }
})
const emit = defineEmits(['close'])

const ov = ref({ balance: 0, cash: 0, rows: [], positions: [], events: [] })
const error = ref('')

const KINDS = { deposit: 'Пополнение', withdraw: 'Вывод', buy: 'Покупка ОВДП', sell: 'Продажа ОВДП' }

const pad = (n) => String(n).padStart(2, '0')
const showDate = (unix) => {
  const d = new Date(unix * 1000)
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`
}
const money = (v) => `${v > 0 ? '+' : v < 0 ? '−' : ''}${formatMoney(Math.abs(v))} ₴`
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')

const load = async () => {
  if (!props.account) return
  try {
    const d = await apiGetJson(`/icu/ledger?account_id=${q(props.account.id)}`)
    ov.value = { balance: d.balance || 0, cash: d.cash || 0, rows: d.rows || [], positions: d.positions || [], events: d.events || [] }
    error.value = ''
  } catch (e) {
    error.value = String(e.message || e)
  }
}

watch(() => props.isOpen, (v) => { if (v) load() })
// баланс/счёт мог измениться после загрузки отчёта
watch(() => props.account?.balance, () => { if (props.isOpen) load() })

const shortDate = (iso) => showDate(Date.parse(iso + 'T12:00:00') / 1000)
const EVENT = { coupon: 'Купон', redemption: 'Погашение' }
const upcoming = () => ov.value.events.filter(e => !e.done)
const upcomingTotal = () => Math.round(upcoming().reduce((a, e) => a + e.amount, 0) * 100) / 100
const received = () => ov.value.events.filter(e => e.done)
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-3xl" @close="emit('close')">
    <div class="p-5 space-y-4 font-mono max-h-[85vh] overflow-y-auto">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-xs font-bold uppercase tracking-wider text-slate-300">Журнал · {{ account?.name }}</div>
          <div class="text-[10px] text-slate-500 truncate">{{ account?.iban || 'IBAN не привязан к алиасу' }}</div>
        </div>
        <div class="text-right shrink-0">
          <div class="text-[10px] uppercase text-slate-500">Баланс</div>
          <div class="text-lg font-bold text-blue-200">{{ formatMoney(ov.balance) }} ₴</div>
          <div v-if="ov.positions.length" class="text-[10px] text-slate-500">свободно {{ formatMoney(ov.cash) }} ₴</div>
        </div>
      </div>

      <!-- позиции -->
      <div v-if="ov.positions.length" class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
              <th class="p-2">ОВДП</th><th class="p-2 text-right">Шт.</th><th class="p-2 text-right">Накоплено</th>
              <th class="p-2 text-right">Стоимость</th><th class="p-2 text-right">Ближайшая выплата</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in ov.positions" :key="p.isin" class="border-b border-slate-800/60">
              <td class="p-2">
                <div class="text-slate-200 font-bold">{{ p.isin }}</div>
                <div v-if="p.no_terms" class="text-[10px] text-amber-400">ставка и погашение не заданы — оценка по цене покупки (алиас → «Облигации»)</div>
                <div v-else class="text-[10px] text-slate-500">{{ p.name }} · {{ p.coupon_rate }}% · до {{ shortDate(p.maturity) }}</div>
              </td>
              <td class="p-2 text-right text-slate-300">{{ p.qty }}</td>
              <td class="p-2 text-right text-slate-400">{{ p.no_terms ? '—' : formatMoney(p.accrued) + ' ₴' }}</td>
              <td class="p-2 text-right font-bold text-blue-200">{{ formatMoney(p.value) }} ₴</td>
              <td class="p-2 text-right text-slate-300 whitespace-nowrap">
                <template v-if="p.next_date">{{ shortDate(p.next_date) }}<div class="text-[10px] text-slate-500">{{ formatMoney(p.next_amount) }} ₴</div></template>
                <template v-else>—</template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- выплаты по графику (считаются, в базе не хранятся) -->
      <details v-if="ov.events.length" class="text-xs">
        <summary class="cursor-pointer text-[10px] uppercase tracking-wider text-slate-500">Выплаты по графику: впереди {{ upcoming().length }} на {{ formatMoney(upcomingTotal()) }} ₴, получено {{ received().length }}</summary>
        <div class="mt-2 space-y-1 max-h-40 overflow-y-auto">
          <div v-for="(e, i) in ov.events" :key="i" class="flex items-center gap-3 text-[11px]" :class="e.done ? 'text-slate-600' : 'text-slate-300'">
            <span class="w-20 whitespace-nowrap">{{ shortDate(e.date) }}</span>
            <span class="w-24">{{ EVENT[e.kind] }}</span>
            <span class="flex-1 truncate">{{ e.isin }}</span>
            <span class="font-bold whitespace-nowrap" :class="!e.done && 'text-emerald-400'">+{{ formatMoney(e.amount) }} ₴</span>
          </div>
        </div>
      </details>

      <!-- журнал из отчёта -->
      <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
              <th class="p-2">Дата</th>
              <th class="p-2">Тип</th>
              <th class="p-2">Заметка</th>
              <th class="p-2 text-right">Сумма</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in ov.rows" :key="r.id" class="border-b border-slate-800/60">
              <td class="p-2 whitespace-nowrap text-slate-400">{{ showDate(r.time) }}</td>
              <td class="p-2 text-slate-300">{{ KINDS[r.kind] || r.kind }}<div v-if="r.isin" class="text-[10px] text-slate-500">{{ r.isin }} × {{ r.qty }}</div></td>
              <td class="p-2 text-slate-400 max-w-[240px] truncate">{{ r.note }}</td>
              <td class="p-2 text-right whitespace-nowrap font-bold" :class="tone(r.amount)">{{ money(r.amount) }}</td>
            </tr>
            <tr v-if="!ov.rows.length">
              <td colspan="4" class="p-6 text-center text-slate-600 text-[11px] uppercase">Записей нет — загрузите брокерский отчёт</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="error" class="text-[11px] text-rose-400">{{ error }}</div>

      <div class="flex justify-end">
        <UiButton variant="cancel" size="sm" @click="emit('close')">Закрыть</UiButton>
      </div>
    </div>
  </UiModal>
</template>

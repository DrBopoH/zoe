<script setup>
import { ref, computed, watch, inject } from 'vue'
import { apiGetJson, apiSend, q } from '../utils/api.js'
import { formatMoney } from '../utils/formatters.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'
import { parseDateText, isoToText } from '../utils/dateText.js'

// Журнал ручного счёта ICU: записи вводятся руками, сверяются с операциями кеша.
// зелёный — есть парная операция на карте; оранжевый — в кеше нет; красный — в кеше лежит другая сумма;
// «призраки» — операции карты, которых в журнале ещё нет (принять → станут записями).
const props = defineProps({
  account: { type: Object, default: null },
  isOpen: { type: Boolean, default: false }
})
const emit = defineEmits(['close'])
const refresh = inject('refreshData', null)

const ov = ref({ balance: 0, cash: 0, rows: [], ghosts: [], positions: [], events: [], bonds: [] })
const error = ref('')
const busy = ref(false)

const KINDS = { deposit: 'Пополнение', withdraw: 'Вывод', buy: 'Покупка ОВДП', sell: 'Продажа ОВДП', adjust: 'Корректировка' }
const isBond = (k) => k === 'buy' || k === 'sell'
const STATUS = {
  green: { dot: 'bg-emerald-400', text: 'Есть на карте' },
  orange: { dot: 'bg-amber-400', text: 'В кеше карты не найдено' },
  red: { dot: 'bg-rose-500', text: 'В кеше другая сумма' }
}

const pad = (n) => String(n).padStart(2, '0')
const dayStr = (unix) => {
  const d = new Date(unix * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
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
    ov.value = { balance: d.balance || 0, cash: d.cash || 0, rows: d.rows || [], ghosts: d.ghosts || [], positions: d.positions || [], events: d.events || [], bonds: d.bonds || [] }
    error.value = ''
  } catch (e) {
    error.value = String(e.message || e)
  }
}

// Форма (добавление / правка)
const form = ref(null) // null — скрыта
const openForm = (row) => {
  form.value = row
    ? { id: row.id, date: isoToText(dayStr(row.time)), kind: row.kind, amount: row.kind === 'adjust' ? row.amount : isBond(row.kind) && row.qty ? Math.round(Math.abs(row.amount) / row.qty * 10000) / 10000 : Math.abs(row.amount), note: row.note || '', src_tx_id: row.src_tx_id || '', isin: row.isin || '', qty: row.qty || null }
    : { id: 0, date: isoToText(dayStr(Date.now() / 1000)), kind: 'deposit', amount: null, note: '', src_tx_id: '', isin: ov.value.bonds[0]?.isin || '', qty: null }
}

watch(() => props.isOpen, (v) => {
  if (v) { form.value = null; load() }
})
// баланс/счёт мог измениться после синка
watch(() => props.account?.balance, () => { if (props.isOpen) load() })

const run = async (fn) => {
  busy.value = true
  error.value = ''
  try {
    await fn()
    await load()
    if (refresh) await refresh()
  } catch (e) {
    error.value = String(e.message || e)
  } finally {
    busy.value = false
  }
}

const saveRow = () => run(async () => {
  const f = form.value
  await apiSend('POST', '/icu/ledger', {
    id: f.id, account_id: props.account.id, date: parseDateText(f.date), kind: f.kind,
    amount: isBond(f.kind) ? Math.round(Number(f.amount) * Number(f.qty) * 100) / 100 : Number(f.amount), note: f.note, src_tx_id: f.src_tx_id,
    isin: isBond(f.kind) ? f.isin : '', qty: isBond(f.kind) ? Number(f.qty) : 0
  })
  form.value = null
})
const removeRow = (row) => run(() => apiSend('DELETE', `/icu/ledger?id=${row.id}`))
const accept = (g) => run(() => apiSend('POST', '/icu/accept', { account_id: props.account.id, tx_ids: [g.tx_id] }))
const acceptAll = () => run(() => apiSend('POST', '/icu/accept', { account_id: props.account.id, all: true }))

const formOk = computed(() => {
  const f = form.value
  if (!f || !parseDateText(f.date) || f.amount === null || f.amount === '' || Number(f.amount) === 0) return false
  return !isBond(f.kind) || (!!f.isin && Number(f.qty) > 0)
})

const shortDate = (iso) => showDate(Date.parse(iso + 'T12:00:00') / 1000)
const EVENT = { coupon: 'Купон', redemption: 'Погашение' }
const upcoming = computed(() => ov.value.events.filter(e => !e.done))
const upcomingTotal = computed(() => Math.round(upcoming.value.reduce((a, e) => a + e.amount, 0) * 100) / 100)
const received = computed(() => ov.value.events.filter(e => e.done))
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-3xl" @close="emit('close')">
    <div class="p-5 space-y-4 font-mono max-h-[85vh] overflow-y-auto">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-xs font-bold uppercase tracking-wider text-slate-300">Журнал · {{ account?.name }}</div>
          <div class="text-[10px] text-slate-500 truncate">{{ account?.iban }}</div>
        </div>
        <div class="text-right shrink-0">
          <div class="text-[10px] uppercase text-slate-500">Баланс</div>
          <div class="text-lg font-bold text-blue-200">{{ formatMoney(ov.balance) }} ₴</div>
          <div v-if="ov.positions.length" class="text-[10px] text-slate-500">свободно {{ formatMoney(ov.cash) }} ₴</div>
        </div>
      </div>

      <!-- призраки: операции карты без записи -->
      <div v-if="ov.ghosts.length" class="border border-rose-500/30 bg-rose-500/5 rounded p-3 space-y-2">
        <div class="flex items-center justify-between gap-2">
          <span class="text-[10px] uppercase tracking-wider text-rose-300 font-bold">Есть на карте, нет в журнале ({{ ov.ghosts.length }})</span>
          <UiButton variant="outline" size="sm" :disabled="busy" @click="acceptAll">Принять все</UiButton>
        </div>
        <div v-for="g in ov.ghosts" :key="g.tx_id" class="flex items-center gap-3 text-[11px]">
          <span class="text-slate-400 whitespace-nowrap">{{ showDate(g.time) }}</span>
          <span class="flex-1 truncate text-slate-300">{{ g.counter_name || g.description || '—' }}</span>
          <span class="font-bold whitespace-nowrap" :class="tone(g.amount)">{{ money(g.amount) }}</span>
          <UiButton variant="subtle" size="sm" :disabled="busy" @click="accept(g)">Принять</UiButton>
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
              <td class="p-2"><div class="text-slate-200 font-bold">{{ p.isin }}</div><div class="text-[10px] text-slate-500">{{ p.name }} · {{ p.coupon_rate }}% · до {{ shortDate(p.maturity) }}</div></td>
              <td class="p-2 text-right text-slate-300">{{ p.qty }}</td>
              <td class="p-2 text-right text-slate-400">{{ formatMoney(p.accrued) }} ₴</td>
              <td class="p-2 text-right font-bold text-blue-200">{{ formatMoney(p.value) }} ₴</td>
              <td class="p-2 text-right text-slate-300 whitespace-nowrap">{{ p.next_date ? shortDate(p.next_date) : '—' }}<div class="text-[10px] text-slate-500">{{ formatMoney(p.next_amount) }} ₴</div></td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- выплаты по графику (считаются, в базе не хранятся) -->
      <details v-if="ov.events.length" class="text-xs">
        <summary class="cursor-pointer text-[10px] uppercase tracking-wider text-slate-500">Выплаты по графику: впереди {{ upcoming.length }} на {{ formatMoney(upcomingTotal) }} ₴, получено {{ received.length }}</summary>
        <div class="mt-2 space-y-1 max-h-40 overflow-y-auto">
          <div v-for="(e, i) in ov.events" :key="i" class="flex items-center gap-3 text-[11px]" :class="e.done ? 'text-slate-600' : 'text-slate-300'">
            <span class="w-20 whitespace-nowrap">{{ shortDate(e.date) }}</span>
            <span class="w-24">{{ EVENT[e.kind] }}</span>
            <span class="flex-1 truncate">{{ e.isin }}</span>
            <span class="font-bold whitespace-nowrap" :class="!e.done && 'text-emerald-400'">+{{ formatMoney(e.amount) }} ₴</span>
          </div>
        </div>
      </details>

      <!-- журнал -->
      <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
              <th class="p-2 w-6"></th>
              <th class="p-2">Дата</th>
              <th class="p-2">Тип</th>
              <th class="p-2">Заметка</th>
              <th class="p-2 text-right">Сумма</th>
              <th class="p-2 w-14"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in ov.rows" :key="r.id" class="border-b border-slate-800/60">
              <td class="p-2">
                <span v-if="r.status" class="inline-block w-2 h-2 rounded-full" :class="STATUS[r.status]?.dot" :title="STATUS[r.status]?.text" />
              </td>
              <td class="p-2 whitespace-nowrap text-slate-400">{{ showDate(r.time) }}</td>
              <td class="p-2 text-slate-300">{{ KINDS[r.kind] || r.kind }}<div v-if="r.isin" class="text-[10px] text-slate-500">{{ r.isin }} × {{ r.qty }}</div></td>
              <td class="p-2 text-slate-400 max-w-[240px]">
                <div class="truncate">{{ r.note }}</div>
                <div v-if="r.status === 'red' && r.mismatch" class="text-[10px] text-rose-400">
                  в кеше: {{ money(r.mismatch.amount) }} · {{ showDate(r.mismatch.time) }}
                </div>
              </td>
              <td class="p-2 text-right whitespace-nowrap font-bold" :class="tone(r.amount)">{{ money(r.amount) }}</td>
              <td class="p-2 whitespace-nowrap text-right">
                <UiIconButton icon="edit" tone="accent" size="sm" @click="openForm(r)" />
                <UiIconButton icon="trash" tone="danger" size="sm" @click="removeRow(r)" />
              </td>
            </tr>
            <tr v-if="!ov.rows.length">
              <td colspan="6" class="p-6 text-center text-slate-600 text-[11px] uppercase">Записей нет — добавьте первое пополнение</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- форма -->
      <div v-if="form" class="border border-slate-800 rounded p-3 space-y-3 bg-slate-950/40">
        <div class="grid grid-cols-1 sm:grid-cols-4 gap-3">
          <label class="space-y-1"><span class="ui-label">Дата</span><input v-model="form.date" inputmode="numeric" placeholder="ДД.ММ.ГГГГ" class="ui-input" :class="form.date && !parseDateText(form.date) && 'ui-input--invalid'" /></label>
          <label class="space-y-1">
            <span class="ui-label">Тип</span>
            <select v-model="form.kind" class="ui-select">
              <option v-for="(n, k) in KINDS" :key="k" :value="k">{{ n }}</option>
            </select>
          </label>
          <label class="space-y-1">
            <span class="ui-label">{{ isBond(form.kind) ? 'Цена за 1 шт., ₴' : form.kind === 'adjust' ? 'Сумма, ₴ (±)' : 'Сумма, ₴' }}</span>
            <input v-model.number="form.amount" type="number" step="0.01" class="ui-input" />
          </label>
          <label class="space-y-1"><span class="ui-label">Заметка</span><input v-model="form.note" class="ui-input" /></label>
        </div>
        <div v-if="isBond(form.kind)" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label class="space-y-1">
            <span class="ui-label">Облигация</span>
            <select v-model="form.isin" class="ui-select">
              <option v-for="b in ov.bonds" :key="b.isin" :value="b.isin">{{ b.isin }} · {{ b.name || b.maturity }}</option>
            </select>
            <span v-if="!ov.bonds.length" class="text-[10px] text-amber-400">сначала добавьте облигацию: карандаш алиаса → IBAN ICU</span>
          </label>
          <label class="space-y-1"><span class="ui-label">Количество, шт.</span><input v-model.number="form.qty" type="number" min="1" step="1" class="ui-input" /></label>
        </div>
        <div v-if="isBond(form.kind)" class="text-[10px] text-slate-500">
          цена за штуку с накопленным купоном; итого {{ form.kind === 'buy' ? '−' : '+' }}{{ formatMoney(Math.round((Number(form.amount) || 0) * (Number(form.qty) || 0) * 100) / 100) }} ₴
        </div>
        <div class="flex justify-end gap-2">
          <UiButton variant="cancel" size="sm" @click="form = null">Отмена</UiButton>
          <UiButton variant="primary" size="sm" :disabled="busy || !formOk" @click="saveRow">Сохранить</UiButton>
        </div>
      </div>

      <div v-if="error" class="text-[11px] text-rose-400">{{ error }}</div>

      <div class="flex items-center justify-between gap-2">
        <div class="flex gap-2">
          <UiButton variant="subtle" size="sm" @click="openForm(null)">+ Запись</UiButton>
        </div>
        <UiButton variant="cancel" size="sm" @click="emit('close')">Закрыть</UiButton>
      </div>
    </div>
  </UiModal>
</template>

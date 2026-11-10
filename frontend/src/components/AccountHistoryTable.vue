<script setup>
import { computed, ref, watch } from 'vue'
import { formatMoney, formatMaskedIBAN } from '../utils/formatters.js'
import { counterpartyName, isPersonTransaction } from '../utils/personLayout.js'
import UiButton from './ui/UiButton.vue'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  // выбранные счета/карты: { id, label, theme }
  selected: { type: Array, default: () => [] }
})
const emit = defineEmits(['clear'])

const PAGE = 100
const visible = ref(PAGE)
const openId = ref(null)

const byId = computed(() => new Map(props.selected.map(a => [a.id, a])))

const rows = computed(() => {
  const ids = byId.value
  return props.transactions
    .filter(t => ids.has(t.account_id))
    .sort((a, b) => new Date(b.time) - new Date(a.time))
})

// при смене выбора начинаем с первой страницы и закрываем раскрытую строку
watch(() => props.selected.map(a => a.id).join('|'), () => {
  visible.value = PAGE
  openId.value = null
})

const shown = computed(() => rows.value.slice(0, visible.value))

const totals = computed(() => {
  let income = 0
  let expense = 0
  for (const t of rows.value) {
    if (t.amount >= 0) income += t.amount
    else expense += t.amount
  }
  const r = (v) => Math.round(v * 100) / 100 + 0 // копейки без хвоста float и без «−0,00»
  return { income: r(income), expense: r(expense), net: r(income + expense) }
})

const CURRENCY = { 980: '₴', 840: '$', 978: '€', 985: 'zł' }
const cur = (t) => CURRENCY[t.currency_code] || (t.currency_code ? String(t.currency_code) : '₴')

const money = (v, t) => `${v > 0 ? '+' : v < 0 ? '−' : ''}${formatMoney(Math.abs(v))} ${cur(t)}`
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')

const pad = (n) => String(n).padStart(2, '0')
const dateStr = (iso) => {
  const d = new Date(iso)
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`
}
const timeStr = (iso) => {
  const d = new Date(iso)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const title = (t) => (t.counter_name || '').trim() || (t.description || '').trim() || '—'
const subtitle = (t) => {
  const d = (t.description || '').trim()
  return d && d !== title(t) ? d : ''
}
const who = (t) => (isPersonTransaction(t) ? counterpartyName(t) : '')
const counterAcct = (t) => (t.counter_iban ? formatMaskedIBAN(t.counter_iban) : '')

const details = (t) => {
  const acc = byId.value.get(t.account_id)
  const list = [
    ['Время', new Date(t.time).toLocaleString('uk-UA')],
    ['Счёт', acc ? acc.fullLabel : t.account_id],
    ['Описание', t.description],
    ['Комментарий', t.comment],
    ['Контрагент', t.counter_name],
    ['Счёт контрагента', t.counter_iban],
    ['MCC', t.mcc || ''],
    ['Сумма операции', t.operation_amount && t.operation_amount !== t.amount ? money(t.operation_amount, t) : ''],
    ['Комиссия', t.commission ? `${formatMoney(t.commission)} ${cur(t)}` : ''],
    ['Кэшбэк', t.cashback ? `${formatMoney(t.cashback)} ${cur(t)}` : ''],
    ['Статус', t.is_hold ? 'Холд (не проведена)' : ''],
    ['ID', t.id]
  ]
  return list.filter(([, v]) => v !== '' && v !== null && v !== undefined)
}

const toggleRow = (id) => { openId.value = openId.value === id ? null : id }
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div class="text-xs font-bold text-emerald-400 uppercase tracking-wider flex items-center gap-2 flex-wrap">
        <span>История операций</span>
        <span v-if="selected.length" class="text-[9px] px-1.5 py-0.2 bg-emerald-500/10 border border-emerald-500/20 rounded text-emerald-400 font-mono">
          {{ rows.length }}
        </span>
        <span v-for="a in selected" :key="a.id" class="text-[9px] px-1.5 py-0.5 rounded border font-mono normal-case" :class="a.theme.tagStyle">
          {{ a.label }}
        </span>
      </div>

      <div v-if="selected.length" class="flex items-center gap-4 text-[11px] font-mono">
        <span class="text-slate-500">Приход <span class="text-emerald-400 font-bold">{{ formatMoney(totals.income) }}</span></span>
        <span class="text-slate-500">Расход <span class="text-rose-400 font-bold">{{ formatMoney(Math.abs(totals.expense)) }}</span></span>
        <span class="text-slate-500">Итого <span class="font-bold" :class="tone(totals.net)">{{ formatMoney(totals.net) }}</span></span>
        <UiButton variant="outline" size="sm" @click="emit('clear')">Снять выбор</UiButton>
      </div>
    </div>

    <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
            <th class="p-3 whitespace-nowrap">ДАТА</th>
            <th v-if="selected.length > 1" class="p-3">КАРТА</th>
            <th class="p-3">ОПЕРАЦИЯ</th>
            <th class="p-3">КОНТРАГЕНТ</th>
            <th class="p-3 text-right">СУММА</th>
          </tr>
        </thead>
        <tbody class="font-mono">
          <template v-for="t in shown" :key="t.id">
            <tr
              class="border-b border-slate-800/60 hover:bg-slate-800/40 cursor-pointer transition-colors"
              :class="openId === t.id && 'bg-slate-800/30'"
              @click="toggleRow(t.id)"
            >
              <td class="p-3 whitespace-nowrap text-slate-400">
                {{ dateStr(t.time) }} <span class="text-slate-600">{{ timeStr(t.time) }}</span>
              </td>
              <td v-if="selected.length > 1" class="p-3">
                <span class="text-[9px] px-1.5 py-0.5 rounded border" :class="byId.get(t.account_id)?.theme.tagStyle">
                  {{ byId.get(t.account_id)?.label }}
                </span>
              </td>
              <td class="p-3 max-w-[360px]">
                <div class="font-bold text-slate-200 truncate">{{ title(t) }}</div>
                <div v-if="subtitle(t)" class="text-[10px] text-slate-500 truncate">{{ subtitle(t) }}</div>
              </td>
              <td class="p-3 whitespace-nowrap">
                <template v-if="who(t) || counterAcct(t)">
                  <div v-if="who(t)" class="text-slate-300">{{ who(t) }}</div>
                  <div v-if="counterAcct(t)" class="text-[10px] text-slate-500">{{ counterAcct(t) }}</div>
                </template>
                <span v-else class="text-slate-600">—</span>
              </td>
              <td class="p-3 text-right whitespace-nowrap font-bold" :class="tone(t.amount)">
                {{ money(t.amount, t) }}
                <span v-if="t.is_hold" class="ml-1 text-[9px] text-amber-400 font-normal">HOLD</span>
              </td>
            </tr>

            <tr v-if="openId === t.id" class="border-b border-slate-800/60 bg-slate-950/50">
              <td :colspan="selected.length > 1 ? 5 : 4" class="p-3">
                <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-[11px] select-text">
                  <template v-for="[k, v] in details(t)" :key="k">
                    <dt class="text-slate-500 uppercase text-[10px] tracking-wider">{{ k }}</dt>
                    <dd class="text-slate-300 break-all">{{ v }}</dd>
                  </template>
                </dl>
              </td>
            </tr>
          </template>

          <tr v-if="!selected.length">
            <td colspan="5" class="p-8 text-center text-slate-600 text-xs uppercase">
              Выберите карту — клик по плитке, можно несколько
            </td>
          </tr>
          <tr v-else-if="rows.length === 0">
            <td colspan="5" class="p-8 text-center text-slate-600 text-xs uppercase">
              По выбранным картам операций нет
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="rows.length > visible" class="flex justify-center">
      <UiButton variant="outline" size="sm" @click="visible += PAGE">
        Показать ещё ({{ Math.min(PAGE, rows.length - visible) }} из {{ rows.length - visible }})
      </UiButton>
    </div>
  </div>
</template>

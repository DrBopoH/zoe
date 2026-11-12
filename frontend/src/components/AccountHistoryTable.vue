<script setup>
import { computed, ref, watch } from 'vue'
import { formatMoney, formatMaskedIBAN, money as fmtMoney, moneySigned, curSym } from '../utils/formatters.js'
import { counterpartyName, isPersonTransaction } from '../utils/personLayout.js'
import UiButton from './ui/UiButton.vue'
import { effectiveTimes, isDateOnly } from '../utils/historyOrder.js'
import { tagsFor, cleanTitle } from '../utils/txTags.js'
import { usersById, personOf, swapAlias } from '../utils/personName.js'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  // справочные строки (покупка/продажа/выплаты облигаций): видны в истории, но в приход/расход не входят
  memo: { type: Array, default: () => [] },
  // выбранные счета/карты: { id, label, theme }
  selected: { type: Array, default: () => [] },
  // выбрана свеча графика: показываем только операции её корзины времени { from, to (мс), title }
  range: { type: Object, default: null },
  // «все счета» (карты не выбраны, фильтр только по свече): без плашек карт и без кнопки снятия выбора
  allScope: { type: Boolean, default: false },
  // лица (id, name, avatar_url): алиас контрагента в строке подменяется на привязанное Лицо
  users: { type: Array, default: () => [] }
})
const emit = defineEmits(['clear', 'clear-range'])

const PAGE = 100
const visible = ref(PAGE)
const openId = ref(null)

const byId = computed(() => new Map(props.selected.map(a => [a.id, a])))

const inRange = (t) => {
  const r = props.range
  if (!r) return true
  const ms = new Date(t.time).getTime()
  return ms >= r.from && ms < r.to
}

// порядок — по логическим правилам внутри дня для записей брокера (см. utils/historyOrder.js), по всем операциям, а не только видимым
const order = computed(() => effectiveTimes(props.transactions, props.memo))

const rows = computed(() => {
  const ids = byId.value
  const at = order.value
  return [...props.transactions, ...props.memo]
    .filter(t => ids.has(t.account_id) && inRange(t))
    .sort((a, b) => at.get(b.id) - at.get(a.id))
})

// при смене выбора начинаем с первой страницы и закрываем раскрытую строку
watch(() => props.selected.map(a => a.id).join('|') + '|' + (props.range ? props.range.key : ''), () => {
  visible.value = PAGE
  openId.value = null
})

const shown = computed(() => rows.value.slice(0, visible.value))

const totals = computed(() => {
  let income = 0
  let expense = 0
  for (const t of rows.value) {
    if (t.internal || t.memo) continue // перевод между своими счетами — не доход и не расход
    if (t.amount >= 0) income += t.amount
    else expense += t.amount
  }
  const r = (v) => Math.round(v * 100) / 100 + 0 // копейки без хвоста float и без «−0,00»
  return { income: r(income), expense: r(expense), net: r(income + expense) }
})

const cur = (t) => curSym(t.currency_code)

const money = (v, t) => moneySigned(v, cur(t))
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

const people = computed(() => usersById(props.users))
const person = (t) => personOf(t, people.value)

const rawTitle = (t) => (t.counter_name || '').trim() || (t.description || '').trim() || '—'
const title = (t) => swapAlias(rawTitle(t), t, person(t))
const subtitle = (t) => {
  const d = (t.description || '').trim()
  return d && d !== rawTitle(t) ? swapAlias(d, t, person(t)) : ''
}
// контрагент: Лицо (аватар + имя), иначе сырое имя
const who = (t) => (person(t) ? person(t).name : isPersonTransaction(t) ? counterpartyName(t) : '')
// счёт/карта контрагента, с которой пришёл или на которую ушёл платёж (маска). Источники по порядку:
//   1. точный: counter_iban, который отдал банк, либо номер карты в самом имени/описании («Грошові перекази: 4874 07** **** 8644»);
//   2. по алиасу: счета и карты сырого алиаса, который подменило Лицо (alias_accts с бэкенда). Один номер — пунктирная рамка;
//      несколько — первый (подтверждённый кешем идёт раньше) и «+N» с подсказкой, где перечислены все.
// Банки отдают карту контрагента не всегда, поэтому по алиасу номер косвенный — он помечен пунктиром.
const CARD_IN_TEXT = /(\d{4})[\d\s]*[*•xX][*•xX\s]*(\d{4})/
const maskId = (v) => (String(v).includes('*') ? String(v) : formatMaskedIBAN(v))
const counterAcct = (t) => {
  const v = (t.counter_iban || '').trim()
  if (v) return { text: maskId(v), exact: true }
  const m = CARD_IN_TEXT.exec(t.counter_name || '') || CARD_IN_TEXT.exec(t.description || '')
  if (m) return { text: `${m[1]}****${m[2]}`, exact: true }
  const list = t.alias_accts || []
  if (!list.length) return null
  const all = list.map((a) => maskId(a.id))
  return {
    text: all[0],
    exact: false,
    more: all.length - 1,
    tip: all.length > 1
      ? `Номер не указан в операции. Счета и карты алиаса:\n${list.map((a, i) => `${all[i]}${a.confirmed ? ' ✓' : ''}`).join('\n')}`
      : 'Номер не указан в операции, взят из привязки алиаса'
  }
}

const details = (t) => {
  const acc = byId.value.get(t.account_id)
  const list = [
    ['Время', new Date(t.time).toLocaleString('uk-UA')],
    ['Счёт', acc ? acc.fullLabel : t.account_id],
    ['Описание', swapAlias(t.description, t, person(t))],
    ['Комментарий', t.comment],
    ['Контрагент', person(t) ? person(t).name : t.counter_name],
    ['Счёт контрагента', t.counter_iban || (counterAcct(t) ? counterAcct(t).text + (counterAcct(t).more ? ` (+${counterAcct(t).more})` : '') : '')],
    ['MCC', t.mcc || ''],
    ['Сумма операции', t.operation_amount && t.operation_amount !== t.amount ? money(t.operation_amount, t) : ''],
    ['Комиссия', t.commission ? fmtMoney(t.commission, cur(t)) : ''],
    ['Кэшбэк', t.cashback ? fmtMoney(t.cashback, cur(t)) : ''],
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
        <span v-if="range" class="text-[9px] px-1.5 py-0.5 rounded border font-mono normal-case bg-sky-500/10 border-sky-500/30 text-sky-300 cursor-pointer" title="Снять фильтр по свече" @click="emit('clear-range')">
          {{ range.title }} ✕
        </span>
        <span v-if="allScope" class="text-[9px] px-1.5 py-0.5 rounded border font-mono normal-case border-slate-700 text-slate-400">все счета</span>
        <span v-for="a in (allScope ? [] : selected)" :key="a.id" class="text-[9px] px-1.5 py-0.5 rounded border font-mono normal-case" :class="a.theme.tagStyle">
          {{ a.label }}
        </span>
      </div>

      <div v-if="selected.length" class="flex items-center gap-4 text-[11px] font-mono">
        <span class="text-slate-500">Приход <span class="text-emerald-400 font-bold">{{ formatMoney(totals.income) }}</span></span>
        <span class="text-slate-500">Расход <span class="text-rose-400 font-bold">{{ formatMoney(Math.abs(totals.expense)) }}</span></span>
        <span class="text-slate-500">Итого <span class="font-bold" :class="tone(totals.net)">{{ formatMoney(totals.net) }}</span></span>
        <UiButton v-if="!allScope" variant="outline" size="sm" @click="emit('clear')">Снять выбор</UiButton>
      </div>
    </div>

    <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
            <th class="p-3 whitespace-nowrap">ДАТА</th>
            <th v-if="selected.length > 1" class="p-3">КАРТА</th>
            <th class="p-3">ОПЕРАЦИЯ</th>
            <th class="p-3">СЧЁТ</th>
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
                {{ dateStr(t.time) }} <span v-if="!isDateOnly(t)" class="text-slate-600">{{ timeStr(t.time) }}</span>
              </td>
              <td v-if="selected.length > 1" class="p-3">
                <span class="text-[9px] px-1.5 py-0.5 rounded border" :class="byId.get(t.account_id)?.theme.tagStyle">
                  {{ byId.get(t.account_id)?.label }}
                </span>
              </td>
              <td class="p-3 max-w-[360px]">
                <div class="font-bold text-slate-200 truncate">
                  <template v-for="(g, i) in tagsFor(t)" :key="i"><span v-if="g.sep" class="mr-1.5 text-[10px] text-slate-500 font-normal">{{ g.label }}</span><span v-else class="mr-1.5 text-[9px] px-1.5 py-0.5 rounded border font-normal" :class="g.cls">{{ g.label }}</span></template>
                  <template v-if="who(t)">
                    <span class="inline-flex w-5 h-5 mr-1.5 align-middle rounded-full bg-slate-800 border border-slate-700 overflow-hidden items-center justify-center text-[9px] font-bold text-slate-400">
                      <img v-if="person(t)?.avatar_url" :src="person(t).avatar_url" class="w-full h-full object-cover" draggable="false" />
                      <span v-else>{{ who(t).charAt(0).toUpperCase() }}</span>
                    </span>{{ who(t) }}
                  </template>
                  <template v-else>{{ cleanTitle(t, title(t)) }}</template>
                </div>
                <div v-if="!who(t) && subtitle(t)" class="text-[10px] text-slate-500 truncate">{{ subtitle(t) }}</div>
              </td>
              <td class="p-3 whitespace-nowrap">
                <span v-if="counterAcct(t)" class="text-[9px] px-1.5 py-0.5 rounded border text-slate-400 font-normal" :class="counterAcct(t).exact ? 'border-slate-700' : 'border-dashed border-slate-600'" :title="counterAcct(t).exact ? 'Счёт или карта контрагента' : counterAcct(t).tip">{{ counterAcct(t).text }}<template v-if="counterAcct(t).more"> <span class="text-slate-500">+{{ counterAcct(t).more }}</span></template></span>
                <span v-else class="text-slate-600">—</span>
              </td>
              <td class="p-3 text-right whitespace-nowrap font-bold" :class="tone(t.amount)">
                <span v-if="['move', 'trade', 'swap'].includes(t.memo)" class="text-slate-600 font-normal">—</span>
                <span v-else :class="(t.internal || t.memo) && 'opacity-60'">{{ money(t.amount, t) }}</span>
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
              Выберите карту (клик по плитке, можно несколько) или свечу на графике
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

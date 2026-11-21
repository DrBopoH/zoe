<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import AccountPanel from './AccountPanel.vue'
import AccountHistoryTable from './AccountHistoryTable.vue'
import WeekTimeline from './WeekTimeline.vue'
import CashEntries from './CashEntries.vue'
import UiLoadError from './ui/UiLoadError.vue'
import { prepareAccount } from '../utils/accountView.js'
import { BADGES, badgeTotals, weekMonday, timelineWeeks, withCash } from '../utils/glovoLedger.js'
import { useWorkspaceLedger } from '../composables/useWorkspaceLedger.js'
import { pushRequestError } from '../composables/useToasts.js'
import { formatMoney, toLocalDateStr } from '../utils/formatters.js'

// Учёт дела: счета, входящие в дело, бейджи на их операциях и понедельные записи номинального дохода.
// Операции не копируются: дело хранит только ссылки на счета и операции и свои пометки.
const props = defineProps({
  workspaceId: { type: String, required: true },
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] }
})

const { linked, marks, weeks, entries, error, load, toggleAccount, setMark, setWeek, removeWeek, addEntry, updateEntry, removeEntry } = useWorkspaceLedger(() => props.workspaceId)
onMounted(load)
watch(() => props.workspaceId, load)

const guard = (title, fn) => async (...args) => {
  try { await fn(...args) } catch (e) { pushRequestError(title, e) }
}
const onToggleAccount = guard('Не удалось изменить счета дела', toggleAccount)
const onMark = guard('Не удалось поставить бейдж', setMark)
const onSaveWeek = guard('Не удалось записать неделю', setWeek)
const onRemoveWeek = guard('Не удалось удалить неделю', removeWeek)
const onRemoveEntry = guard('Не удалось удалить запись', removeEntry)
// сохранение наличной записи: true — удалось, при ошибке тост и окно остаётся открытым
const onSaveEntry = async (e) => {
  try { await (e.id ? updateEntry(e) : addEntry(e)); return true } catch (err) { pushRequestError('Не удалось сохранить запись', err); return false }
}

const cards = computed(() => props.accounts.map(prepareAccount))
const isLinked = (c) => linked.value.has(c.id)

const toRow = (a) => ({ id: a.id, label: a.pans[0] || a.displayIban, fullLabel: [a.displayIban, ...a.pans].join(' · '), theme: a.theme })
const ownRows = computed(() => cards.value.map(toRow))
const linkedRows = computed(() => cards.value.filter(isLinked).map(toRow))

// операции из выписок и наличные записи считаются вместе
const merged = computed(() => withCash(props.transactions, marks.value, entries.value))
// наличные записи под своей операцией в истории; чья операция не видна (нет в кеше или счёт не в деле) — отдельным списком
const visibleTx = computed(() => new Set(props.transactions.filter((t) => linked.value.has(t.account_id)).map((t) => t.id)))
const cashByTx = computed(() => {
  const m = new Map()
  for (const e of entries.value) {
    if (!e.tx_id || !visibleTx.value.has(e.tx_id)) continue
    m.set(e.tx_id, [...(m.get(e.tx_id) || []), e])
  }
  return m
})
const orphans = computed(() => entries.value.filter((e) => !e.tx_id || !visibleTx.value.has(e.tx_id)))
const totals = computed(() => badgeTotals(merged.value.transactions, merged.value.marks))

// Таймлайн номинального дохода: номинал недели сверяется с приходом следующей недели
const today = computed(() => toLocalDateStr(new Date()))
const currentWeek = computed(() => weekMonday(today.value))
const back = ref(0) // на сколько недель продлена лента назад («Раньше»)
const timeline = computed(() => timelineWeeks({ weeks: weeks.value, transactions: merged.value.transactions, marks: merged.value.marks, today: today.value, back: back.value }))
const tone = (v) => (v > 0 ? 'text-emerald-400' : v < 0 ? 'text-rose-400' : 'text-slate-400')
</script>

<template>
  <div class="space-y-6 font-mono select-none">
    <UiLoadError v-if="error" :message="error" @retry="load" />

    <section class="space-y-3">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Счета дела</div>
      <p class="text-[11px] text-slate-500">Отметь счета и карты, через которые идут деньги дела, в том числе те, с которых были только расходы. Бейджи ставятся на операциях этих счетов.</p>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <AccountPanel
          v-for="c in cards"
          :key="c.key"
          :card="c"
          selectable
          :selected="isLinked(c)"
          role="checkbox"
          tabindex="0"
          :aria-checked="isLinked(c)"
          @click="onToggleAccount(c.id)"
          @keydown.enter.prevent="onToggleAccount(c.id)"
          @keydown.space.prevent="onToggleAccount(c.id)"
        />
      </div>
    </section>

    <section class="space-y-3">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Итоги по бейджам</div>
      <div class="flex flex-wrap gap-2" data-testid="badge-totals">
        <div v-for="b in BADGES" :key="b.key" class="px-3 py-2 rounded border bg-slate-900/60 min-w-[8.5rem]" :class="b.cls.split(' ').filter((x) => x.startsWith('border')).join(' ')">
          <div class="text-[10px] uppercase tracking-wider font-bold" :class="b.cls.split(' ').filter((x) => x.startsWith('text')).join(' ')">{{ b.label }}</div>
          <div class="text-sm font-bold tabular-nums" :class="tone(totals[b.key].sum)">{{ formatMoney(totals[b.key].sum) }}</div>
          <div class="text-[10px] text-slate-500">{{ totals[b.key].count }} опер.</div>
        </div>
      </div>
    </section>

    <CashEntries v-if="orphans.length" :entries="orphans" @remove="onRemoveEntry" />

    <section class="space-y-3">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Номинальный доход по неделям</div>
      <p class="text-[11px] text-slate-500">Номинал — то, что Glovo записал за неделю до налога и вычетов. Деньги приходят на карту через 2–4 дня после отчёта, то есть на следующей неделе (бейджи «Доход» и «Выплата»); разница показывается как удержанное.</p>
      <WeekTimeline :rows="timeline" :current-week="currentWeek" @save="onSaveWeek" @remove="onRemoveWeek" @older="back += 8" />
    </section>

    <AccountHistoryTable
      markable
      :clearable="false"
      empty-hint="Отметь счета дела выше: здесь появятся их операции"
      :badges="BADGES"
      :marks="marks"
      cashable
      :cash="cashByTx"
      :save-cash="onSaveEntry"
      :users="users"
      :own="ownRows"
      :transactions="transactions"
      :selected="linkedRows"
      @mark="onMark"
      @remove-cash="onRemoveEntry"
    />
  </div>
</template>

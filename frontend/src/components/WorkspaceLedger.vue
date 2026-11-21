<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import AccountPanel from './AccountPanel.vue'
import AccountHistoryTable from './AccountHistoryTable.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiLoadError from './ui/UiLoadError.vue'
import { prepareAccount } from '../utils/accountView.js'
import { BADGES, badgeTotals, weekMonday, weekSunday, nominalTotal } from '../utils/glovoLedger.js'
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

const { linked, marks, weeks, error, load, toggleAccount, setMark, setWeek, removeWeek } = useWorkspaceLedger(() => props.workspaceId)
onMounted(load)
watch(() => props.workspaceId, load)

const guard = (title, fn) => async (...args) => {
  try { await fn(...args) } catch (e) { pushRequestError(title, e) }
}
const onToggleAccount = guard('Не удалось изменить счета дела', toggleAccount)
const onMark = guard('Не удалось поставить бейдж', setMark)
const onRemoveWeek = guard('Не удалось удалить неделю', removeWeek)

const cards = computed(() => props.accounts.map(prepareAccount))
const isLinked = (c) => linked.value.has(c.id)

const toRow = (a) => ({ id: a.id, label: a.pans[0] || a.displayIban, fullLabel: [a.displayIban, ...a.pans].join(' · '), theme: a.theme })
const ownRows = computed(() => cards.value.map(toRow))
const linkedRows = computed(() => cards.value.filter(isLinked).map(toRow))

const totals = computed(() => badgeTotals(props.transactions, marks.value))

// Недельные записи: дата любого дня недели → её понедельник
const weekDate = ref(toLocalDateStr(new Date()))
const weekNominal = ref('')
const monday = computed(() => weekMonday(weekDate.value))
const nominalNum = computed(() => Number(String(weekNominal.value).replace(',', '.')))
const canAddWeek = computed(() => !!monday.value && weekNominal.value !== '' && Number.isFinite(nominalNum.value) && nominalNum.value >= 0)
const addWeek = guard('Не удалось записать неделю', async () => {
  if (!canAddWeek.value) return
  await setWeek(monday.value, nominalNum.value)
  weekNominal.value = ''
})
const fmtDay = (iso) => `${iso.slice(8, 10)}.${iso.slice(5, 7)}`
const weekLabel = (w) => `${fmtDay(w)} — ${fmtDay(weekSunday(w))}`
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

    <section class="space-y-3">
      <div class="flex items-baseline gap-3">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">Номинальный доход по неделям</div>
        <div v-if="weeks.length" class="text-[11px] text-slate-500">всего <span class="text-slate-200 font-bold">{{ formatMoney(nominalTotal(weeks)) }}</span></div>
      </div>
      <p class="text-[11px] text-slate-500">Сколько Glovo записал за неделю до налога и вычетов. Выбери любой день недели и сумму, запись ляжет на её понедельник; та же неделя перезаписывается.</p>
      <div class="flex flex-wrap items-center gap-2">
        <input v-model="weekDate" type="date" class="ui-input !w-auto" aria-label="День недели" />
        <span v-if="monday" class="text-[11px] text-slate-500">неделя {{ weekLabel(monday) }}</span>
        <input v-model="weekNominal" type="text" inputmode="decimal" class="ui-input !w-32" placeholder="Сумма, ₴" aria-label="Номинальный доход" @keydown.enter.prevent="addWeek" />
        <UiButton variant="primary" size="sm" :disabled="!canAddWeek" @click="addWeek">Записать</UiButton>
      </div>
      <div v-if="weeks.length" class="bg-slate-900/60 border border-slate-800 rounded divide-y divide-slate-800/60" data-testid="weeks">
        <div v-for="w in [...weeks].reverse()" :key="w.week" class="flex items-center justify-between gap-3 px-3 py-1.5 text-xs">
          <span class="text-slate-400">{{ weekLabel(w.week) }}</span>
          <span class="flex items-center gap-2">
            <span class="font-bold text-slate-200 tabular-nums">{{ formatMoney(w.nominal) }}</span>
            <UiIconButton icon="trash" tone="danger" size="sm" title="Удалить запись недели" @click="onRemoveWeek(w.week)" />
          </span>
        </div>
      </div>
    </section>

    <AccountHistoryTable
      markable
      :clearable="false"
      empty-hint="Отметь счета дела выше: здесь появятся их операции"
      :badges="BADGES"
      :marks="marks"
      :users="users"
      :own="ownRows"
      :transactions="transactions"
      :selected="linkedRows"
      @mark="onMark"
    />
  </div>
</template>

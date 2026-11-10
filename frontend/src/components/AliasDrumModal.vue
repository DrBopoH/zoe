<script setup>
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { buildPersonLayout, idKey, isIbanLike, isPanLike, reliableKey, looseKey, detectBankKey } from '../utils/personLayout.js'
import { prepareAccount } from '../utils/accountView.js'
import { buildRow, matchesCard, passesFilter } from '../utils/resolveRows.js'
import { formatMaskedIBAN } from '../utils/formatters.js'
import { bankTheme } from '../utils/bankTheme.js'
import { apiSend, q } from '../utils/api.js'
import UiIconButton from './ui/UiIconButton.vue'
import CardDrum from './CardDrum.vue'
import TxResolveTable from './TxResolveTable.vue'
import CardTilesPanel from './CardTilesPanel.vue'

// Резолвер связей. Центр — алиас: он ограничивает барабан картами (IBAN), подключёнными к нему.
// Барабан выбирает карту → её история. Справа все карты и PAN-сироты: перетащил на рядок контрагента —
// карта привязана к алиасу этого контрагента.
const props = defineProps({
  isOpen: { type: Boolean, default: false },
  alias: { type: String, default: '' },
  cached: { type: Boolean, default: false },
  graph: { type: Object, default: null },
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] }
})
const emit = defineEmits(['close', 'changed'])

// ── барабан: только счета самого алиаса, у которых есть выписка в кэше
const aliasCards = computed(() => {
  if (!props.isOpen || !props.graph || !props.alias) return []
  const seen = new Set()
  return buildPersonLayout({ aliases: [props.alias] }, props.graph).groups
    .flatMap(g => g.tiles)
    .map(t => t.cacheAcc)
    .filter(a => a && !seen.has(a.id) && seen.add(a.id))
    .map(prepareAccount)
})
const sel = ref(0)
watch(() => [props.isOpen, props.alias], () => { sel.value = 0 }, { immediate: true })
const current = computed(() => aliasCards.value[sel.value] || null)

// ── история выбранной карты
const showAll = ref(false)
const byAccount = computed(() => {
  const m = new Map()
  for (const t of props.transactions) {
    if (!m.has(t.account_id)) m.set(t.account_id, [])
    m.get(t.account_id).push(t)
  }
  return m
})
const rows = computed(() => {
  if (!props.isOpen || !current.value) return []
  const list = (byAccount.value.get(current.value.id) || [])
    .filter(t => passesFilter(t, showAll.value))
    .sort((a, b) => new Date(b.time) - new Date(a.time))
    .map(t => buildRow(t, props.graph))
  return list
})
const totals = computed(() => rows.value.reduce((s, r) => s + r.t.amount, 0))

// ── правая панель: все карты кэша + PAN-сироты
const allCards = computed(() => (props.isOpen ? props.accounts.map(prepareAccount) : []))

const orphanCards = computed(() => {
  if (!props.isOpen) return []
  const ownRel = new Set(), ownLoose = new Set()
  props.accounts.forEach(a => {
    const pans = prepareAccount(a).ids.filter(isPanLike)
    pans.forEach(p => { const r = reliableKey(p); if (r) ownRel.add(r); const l = looseKey(p); if (l) ownLoose.add(l) })
  })
  const out = new Map()
  const add = (p) => {
    if (!isPanLike(p)) return
    const r = reliableKey(p)
    if (r && ownRel.has(r)) return
    const l = looseKey(p)
    if (!r && l && ownLoose.has(l)) return
    const k = r || l || idKey(p)
    if (k && !out.has(k)) out.set(k, p)
  }
  props.transactions.forEach(t => add(t.counter_iban))
  props.graph?.nodes.forEach(n => n.effective.forEach(e => add(e.id)))
  return [...out.entries()].map(([k, p]) => ({
    key: 'pan:' + k,
    id: null,
    balance: null,
    displayIban: 'КАРТА',
    pans: [formatMaskedIBAN(p)],
    ids: [p],
    theme: bankTheme(detectBankKey(p), '')
  }))
})

// какие алиасы уже владеют картой (сохранено в статике)
const ownersByKey = computed(() => {
  const m = new Map()
  props.graph?.nodes.forEach(n => n.effective.forEach(e => {
    if (!e.persisted) return
    if (!m.has(e.key)) m.set(e.key, new Set())
    m.get(e.key).add(n.name)
  }))
  return m
})
const aliasesOf = (c) => {
  const s = new Set()
  c.ids.forEach(id => ownersByKey.value.get(idKey(id))?.forEach(n => s.add(n)))
  return [...s]
}

// ── drag & drop → привязка
const dragCard = ref(null)
const compat = (r) => matchesCard(r.t, dragCard.value)
const linkTarget = (c) => c.ids.find(isIbanLike) || c.ids.find(isPanLike) || ''

const undo = ref(null)
let undoTimer = 0
const busy = ref(false)
const err = ref('')

const linkOne = (card, r) => {
  const identifier = card && linkTarget(card)
  if (!identifier || !r.isCp) return null
  return apiSend('POST', '/users/link-account', {
    user_id: r.userId || '',
    account_identifier: identifier,
    parent_identifier: '',
    note: r.name
  }).then(() => ({ alias: r.name, identifier, label: formatMaskedIBAN(identifier) }))
}
const linkMany = async (pairs) => {
  if (busy.value || !pairs.length) return
  busy.value = true
  err.value = ''
  const done = []
  try {
    for (const [card, r] of pairs) {
      const u = await linkOne(card, r)
      if (u) done.push(u)
    }
  } catch (e) {
    err.value = String(e.message || e)
  } finally {
    busy.value = false
    if (done.length) {
      undo.value = done
      clearTimeout(undoTimer)
      undoTimer = setTimeout(() => { undo.value = null }, 6000)
      emit('changed')
    }
  }
}
const onDropRow = (r) => {
  const card = dragCard.value
  dragCard.value = null
  return linkMany([[card, r]])
}
const doUndo = async () => {
  const list = undo.value
  if (!list) return
  undo.value = null
  try {
    for (const u of list) await apiSend('DELETE', `/users/link-account?identifier=${q(u.identifier)}&alias=${q(u.alias)}`)
    emit('changed')
  } catch (e) {
    err.value = String(e.message || e)
  }
}

// Esc
const onKey = (e) => { if (e.key === 'Escape') emit('close') }
watch(() => props.isOpen, (v) => {
  if (v) window.addEventListener('keydown', onKey)
  else { window.removeEventListener('keydown', onKey); undo.value = null; err.value = '' }
}, { immediate: true })
onBeforeUnmount(() => { window.removeEventListener('keydown', onKey); clearTimeout(undoTimer) })
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="isOpen" class="alias-drum-overlay" data-modal @click.self="emit('close')">
        <UiIconButton class="alias-drum-close" icon="close" title="Закрыть (Esc)" @click="emit('close')" />

        <div class="alias-drum-grid">
          <!-- алиас + барабан -->
          <div class="flex flex-col gap-2 min-w-0">
            <div
              class="px-3.5 py-2.5 rounded border font-bold text-sm text-center break-words backdrop-blur-sm"
              :class="cached ? 'bg-emerald-950/60 border-emerald-500/40 text-emerald-400' : 'bg-amber-950/60 border-amber-500/40 text-amber-400'"
            >{{ alias || '—' }}</div>
            <CardDrum v-if="aliasCards.length" v-model="sel" :cards="aliasCards" />
            <div v-else class="drum-empty">У алиаса нет своих счетов с выпиской в кэше</div>
          </div>

          <!-- история карты -->
          <section class="panel">
            <header class="panel-head">
              <span class="truncate">{{ current ? (current.pans[0] || current.displayIban) : '—' }}</span>
              <span class="text-[9px] px-1.5 py-0.5 bg-emerald-500/10 border border-emerald-500/20 rounded text-emerald-400">{{ rows.length }}</span>
              <label class="ml-auto flex items-center gap-1.5 text-[10px] normal-case tracking-normal text-slate-400 cursor-pointer select-none">
                <input v-model="showAll" type="checkbox" class="accent-emerald-500" />
                все операции (магазины, возвраты)
              </label>
            </header>
            <TxResolveTable
              :rows="rows"
              :dragging="!!dragCard"
              :compat="compat"
              :empty="current ? 'Нет операций по фильтру' : 'Выберите карту'"
              @drop-row="onDropRow"
            />
            <footer class="panel-foot">
              <span v-if="err" class="text-rose-400 truncate">{{ err }}</span>
              <span v-else-if="dragCard">Отпусти на контрагента — карта привяжется к его алиасу</span>
              <span v-else>Перетащи карту справа на рядок контрагента</span>
              <span class="ml-auto font-bold" :class="totals >= 0 ? 'text-emerald-400' : 'text-rose-400'">Итого {{ totals.toFixed(2) }}</span>
            </footer>
          </section>

          <!-- все карты и сироты -->
          <CardTilesPanel
            :cards="allCards"
            :orphans="orphanCards"
            :aliases-of="aliasesOf"
            @dragstart="dragCard = $event"
            @dragend="dragCard = null"
          />
        </div>

        <Transition name="fade">
          <div v-if="undo" class="undo">
            Привязано: {{ undo.length === 1 ? `${undo[0].label} → ${undo[0].alias}` : undo.length }}
            <button class="undo-btn" @click="doUndo">Отменить</button>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.alias-drum-overlay {
  position: fixed;
  inset: 0;
  z-index: 25; /* выше плиток (z-20), но ниже шапки (z-30): шапка остаётся сверху и не блюрится */
  width: 100vw;
  height: 100vh;
  height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4.5rem 1rem 1rem;
  background: rgb(2 6 23 / 0.55);
  backdrop-filter: blur(14px);
  font-family: ui-monospace, monospace;
  user-select: none;
}
.alias-drum-close { position: absolute; top: 4rem; right: 1rem; }
.alias-drum-grid {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr) 390px;
  align-items: start;
  gap: 1rem;
  width: 100%;
  max-width: 1750px;
  height: 590px;
}
.panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: rgb(15 23 42); /* непрозрачный */
  border: 1px solid rgb(30 41 59);
  border-radius: 0.5rem;
  overflow: hidden;
}
.panel-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  height: 40px;
  padding: 0 0.75rem;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgb(52 211 153);
  border-bottom: 1px solid rgb(30 41 59);
}
.panel-foot {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  height: 36px;
  padding: 0 0.75rem;
  font-size: 11px;
  color: rgb(100 116 139);
  border-top: 1px solid rgb(30 41 59);
}
.drum-empty {
  height: 520px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 1rem;
  font-size: 11px;
  color: rgb(100 116 139);
}
.undo {
  position: absolute;
  bottom: 1.5rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.9rem;
  font-size: 12px;
  color: rgb(167 243 208);
  background: rgb(6 78 59 / 0.95);
  border: 1px solid rgb(16 185 129 / 0.6);
  border-radius: 0.5rem;
}
.undo-btn {
  font-weight: 700;
  text-transform: uppercase;
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid rgb(110 231 183);
  color: rgb(236 253 245);
}
.undo-btn:hover { background: rgb(16 185 129 / 0.3); }
</style>

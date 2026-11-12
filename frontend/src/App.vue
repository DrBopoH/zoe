<script setup>
import { ref, onMounted, onUnmounted, computed, watch, provide, nextTick } from 'vue'
import AppHeader from './components/AppHeader.vue'
import SideMenu from './components/SideMenu.vue'
import ScrollJumpButtons from './components/ScrollJumpButtons.vue'
import ToastStack from './components/ToastStack.vue'
import { pushToast } from './composables/useToasts.js'
import { useWheelScroll } from './composables/useWheelScroll.js'
import SmetaView from './views/SmetaView.vue'
import MarketView from './views/MarketView.vue'
import DataView from './views/DataView.vue'
import { cleanPersonName, isP2PTransaction } from './utils/formatters.js'
import { apiGetJson, apiSend } from './utils/api.js'
import { storageGet, storageSet } from './utils/storage.js'
import { resolveNav } from './utils/nav.js'
import { marketNotice } from './utils/marketNotice.js'
import { syncOkLines } from './utils/syncNotice.js'
import { isSyncing, autoSave, syncPath } from './composables/useSync.js'
import { marketPanelOpen } from './utils/marketPanel.js'
import { loadMasks } from './composables/useIbanMasks.js'

const STORAGE_ACTIVE_TAB = 'economic_active_tab'

// Старые значения («Операционная»/«Лица») переводятся в «Учет» + раздел и сразу переписываются в новом виде
const nav = resolveNav(storageGet(STORAGE_ACTIVE_TAB), storageGet('economic_data_section'))
storageSet(STORAGE_ACTIVE_TAB, nav.tab)
storageSet('economic_data_section', nav.section)
const activeTab = ref(nav.tab)
watch(activeTab, (newTab) => storageSet(STORAGE_ACTIVE_TAB, newTab))

// Вкладки живут в KeepAlive (выбор счетов, свеча, раскрытые строки не сбрасываются). У каждой вкладки своя позиция прокрутки: уходя, запоминаем, возвращаясь — восстанавливаем (новая вкладка — с верха)
const tabScroll = {}
watch(activeTab, (_, oldTab) => { if (oldTab) tabScroll[oldTab] = window.scrollY }, { flush: 'pre' })
watch(activeTab, async (tab) => {
  await nextTick()
  window.scrollTo(0, tabScroll[tab] || 0)
})

useWheelScroll() // колесо над слайдерами и горизонтальными областями

const accounts = ref([])
const transactions = ref([])
const users = ref([]) // лица: имя и аватар подменяют алиас контрагента в истории

// Версия данных на бэкенде: растёт при любой записи (синк, импорт, правки лиц).
// Данные перезапрашиваем только когда версия изменилась.
const dataVersion = ref(null)

const errorMsg = ref('')

// Структурированный прогресс синка (клиенты → счета → таймлайн) из /sync/status
const progress = ref(null)
let lastFinishedAt = null // null — ещё не видели ни одного ответа

// Итог синка. Успех: главное окно уходит само, остаётся пуш «Снято» (по таймеру).
// Ошибка: главное окно остаётся красным и не пропадает, пока его не закроют.
const notifyResult = (last) => {
  if (!last) return
  if (lastFinishedAt === null) { lastFinishedAt = last.finished_at; return } // старый итог после перезагрузки страницы не показываем
  if (last.finished_at <= lastFinishedAt) return
  lastFinishedAt = last.finished_at
  if (!last.errors || last.errors.length === 0) {
    pushToast({ kind: 'ok', title: 'Снято', lines: syncOkLines(last), ttl: 6000 })
  }
}

// Рыночные данные (ОВДП, US Treasury) — отдельный процесс рядом с банками: свои итоги и по тосту на источник
let marketSeenAt = null
const notifyMarket = (last) => {
  const r = marketNotice(marketSeenAt, last)
  marketSeenAt = r.seenAt
  r.toasts.forEach(pushToast)
}

// Окно процесса рыночных данных: висит, пока идёт опрос; крестик глушит именно этот запуск (started_at)
const market = ref(null)
const marketDismissed = ref(null)
const marketPanel = computed(() => (marketPanelOpen(market.value, marketDismissed.value) ? market.value : null))
const closeMarket = () => { marketDismissed.value = market.value?.started_at ?? null }

// какой запуск синка пользователь закрыл крестиком (started_at)
const dismissedRun = ref(null)
const panelState = computed(() => {
  const p = progress.value
  if (!p || !p.clients || p.clients.length === 0 && !p.running) return null
  if (dismissedRun.value === p.started_at) return null
  if (p.running) return 'processing'
  return p.last && p.last.errors && p.last.errors.length ? 'error' : null
})
const closePanel = () => { dismissedRun.value = progress.value?.started_at ?? null }

let pollTimer = null

const personsCount = computed(() => {
  const set = new Set()
  transactions.value.forEach(t => {
    if (!isP2PTransaction(t)) return
    const name = cleanPersonName(t.counter_name || t.description)
    if (name) set.add(name.toLowerCase())
  })
  return set.size
})

// Связь с бэкендом пропала (нет ответа на /sync/status). Это НЕ «синк упал»: обход на бэкенде мог продолжать работу,
// поэтому состояние синка не трогаем (кнопка заблокирована, окно процессинга остаётся) и показываем отдельное предупреждение.
const connLostSince = ref(null) // ms, null — связь есть

const fetchStatus = async () => {
  try {
    const data = await apiGetJson('/sync/status')
    connLostSince.value = null
    isSyncing.value = !!data.running
    progress.value = data.progress || null
    if (lastFinishedAt === null && !data.progress?.last) lastFinishedAt = 0
    notifyResult(data.progress?.last)
    if (marketSeenAt === null && !data.market?.last) marketSeenAt = 0
    market.value = data.market || null
    notifyMarket(data.market?.last)

    const ver = data.stats?.data_version
    if (typeof ver === 'number' && ver !== dataVersion.value) {
      dataVersion.value = ver
      await loadData()
    }
  } catch (err) {
    console.error('Status fetch error:', err)
    if (connLostSince.value === null) connLostSince.value = Date.now()
  }
}

const loadData = async () => {
  try {
    // Каждый запрос независим: упавший не мешает остальным подставить свои данные
    const [acc, tx, usr] = await Promise.allSettled([apiGetJson('/accounts'), apiGetJson('/transactions'), apiGetJson('/users')])
    if (usr.status === 'fulfilled') users.value = usr.value || []
    if (acc.status === 'fulfilled') accounts.value = acc.value || []
    if (tx.status === 'fulfilled') transactions.value = tx.value || []
    const failed = [acc, tx, usr].find((r) => r.status === 'rejected')
    errorMsg.value = failed ? String(failed.reason?.message || failed.reason) : ''
    loadMasks() // новые счета после синхронизации получают скрытый вид на бэкенде
  } catch (err) {
    errorMsg.value = err.message
  }
}

// Ручное обновление (после импорта, правок лиц): данные + статус (статус сам подтянет данные при смене версии)
const fetchData = async () => {
  await loadData()
  await fetchStatus()
}

provide('refreshData', fetchData)

const startFullSync = async () => {
  if (isSyncing.value) return
  errorMsg.value = ''
  try {
    await apiSend('POST', syncPath(autoSave.value))
    await fetchStatus()
  } catch (err) {
    errorMsg.value = err.message
  }
}

provide('startSync', startFullSync) // кнопки «Разведать» во вкладке Смета и в окне «Выписки»

onMounted(() => {
  fetchData()

  // Поллим только лёгкий статус; тяжёлые данные грузятся при смене data_version
  pollTimer = setInterval(fetchStatus, 2000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 font-mono flex flex-col p-6 pt-24 sm:pt-20 select-none">
    <SideMenu @refresh="fetchData" />
    <ScrollJumpButtons />
    <ToastStack :progress="progress" :panel-state="panelState" :market="marketPanel" :lost-since="connLostSince" @close-panel="closePanel" @close-market="closeMarket" />

    <!-- Шапка прибита к верху экрана: страница скроллится под ней -->
    <div class="app-bar">
      <div class="max-w-6xl w-full mx-auto px-6 pl-16 xl:pl-6 py-3">
        <AppHeader
          v-model:activeTab="activeTab"
          :is-syncing="isSyncing"
        />
      </div>
    </div>

    <div class="max-w-6xl w-full mx-auto space-y-8">
      <div v-if="errorMsg" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">
        ERROR: {{ errorMsg }}
      </div>

      <KeepAlive><SmetaView v-if="activeTab === 'dashboard'" :accounts="accounts" :transactions="transactions" /></KeepAlive>
      <KeepAlive><MarketView v-if="activeTab === 'market'" :accounts="accounts" :data-version="dataVersion" :running="!!market?.running" /></KeepAlive>
      <KeepAlive><DataView v-if="activeTab === 'data'" :accounts="accounts" :transactions="transactions" :users="users" :data-version="dataVersion" :persons-count="personsCount" @refresh="fetchData" /></KeepAlive>
    </div>
  </div>
</template>
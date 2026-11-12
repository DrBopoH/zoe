<script setup>
import { ref, onMounted, onUnmounted, computed, watch, provide } from 'vue'
import AppHeader from './components/AppHeader.vue'
import SideMenu from './components/SideMenu.vue'
import ScrollJumpButtons from './components/ScrollJumpButtons.vue'
import ToastStack from './components/ToastStack.vue'
import { pushToast } from './composables/useToasts.js'
import { useWheelScroll } from './composables/useWheelScroll.js'
import SmetaView from './views/SmetaView.vue'
import OperationalView from './views/OperationalView.vue'
import PersonsView from './views/PersonsView.vue'
import { cleanPersonName, isP2PTransaction } from './utils/formatters.js'
import { API_BASE } from './utils/api.js'
import { loadMasks } from './composables/useIbanMasks.js'

const STORAGE_ACTIVE_TAB = 'economic_active_tab'

const savedTab = localStorage.getItem(STORAGE_ACTIVE_TAB)
const activeTab = ref(savedTab || 'dashboard')

watch(activeTab, (newTab) => {
  try {
    localStorage.setItem(STORAGE_ACTIVE_TAB, newTab)
  } catch (err) {
    console.error('Save active tab error:', err)
  }
})

useWheelScroll() // колесо над слайдерами и горизонтальными областями

const accounts = ref([])
const transactions = ref([])
const users = ref([]) // лица: имя и аватар подменяют алиас контрагента в истории
const isSyncing = ref(false)

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
    pushToast({ kind: 'ok', title: 'Снято', lines: [`Запросов к банку: ${last.requests}`], ttl: 6000 })
  }
}

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
    const res = await fetch(`${API_BASE}/sync/status`)
    if (!res.ok) throw new Error(`status ${res.status}`)
    const data = await res.json()
    connLostSince.value = null
    isSyncing.value = !!data.running
    progress.value = data.progress || null
    if (lastFinishedAt === null && !data.progress?.last) lastFinishedAt = 0
    notifyResult(data.progress?.last)

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
    const [accRes, txRes, userRes] = await Promise.all([
      fetch(`${API_BASE}/accounts`),
      fetch(`${API_BASE}/transactions`),
      fetch(`${API_BASE}/users`)
    ])
    if (userRes.ok) users.value = (await userRes.json()) || []
    if (accRes.ok) accounts.value = (await accRes.json()) || []
    if (txRes.ok) transactions.value = (await txRes.json()) || []
    errorMsg.value = ''
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
    const res = await fetch(`${API_BASE}/sync/full`, { method: 'POST' })
    if (!res.ok) throw new Error(await res.text())
    await fetchStatus()
  } catch (err) {
    errorMsg.value = err.message
  }
}

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
    <ToastStack :progress="progress" :panel-state="panelState" :lost-since="connLostSince" @close-panel="closePanel" />

    <!-- Шапка прибита к верху экрана: страница скроллится под ней -->
    <div class="app-bar">
      <div class="max-w-6xl w-full mx-auto px-6 pl-16 xl:pl-6 py-3">
        <AppHeader
          v-model:activeTab="activeTab"
          :is-syncing="isSyncing"
          :persons-count="personsCount"
          @sync="startFullSync"
        />
      </div>
    </div>

    <div class="max-w-6xl w-full mx-auto space-y-8">
      <div v-if="errorMsg" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">
        ERROR: {{ errorMsg }}
      </div>

      <SmetaView v-if="activeTab === 'dashboard'" :accounts="accounts" :transactions="transactions" />
      <OperationalView v-if="activeTab === 'accounts'" :accounts="accounts" :transactions="transactions" :users="users" />
      <PersonsView v-if="activeTab === 'entities'" :transactions="transactions" :accounts="accounts" :data-version="dataVersion" @refresh="fetchData" />
    </div>
  </div>
</template>
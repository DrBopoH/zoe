<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import AppHeader from './components/AppHeader.vue'
import SmetaView from './views/SmetaView.vue'
import OperationalView from './views/OperationalView.vue'
import PersonsView from './views/PersonsView.vue'
import { cleanPersonName, isP2PTransaction } from './utils/formatters.js'

const API_BASE = 'http://localhost:8080/api'
const STORAGE_ACTIVE_TAB = 'economic_active_tab'

// Восстановление сохраненной вкладки или дефолт 'dashboard'
const savedTab = localStorage.getItem(STORAGE_ACTIVE_TAB)
const activeTab = ref(savedTab || 'dashboard')

// Персистентность при смене вкладки
watch(activeTab, (newTab) => {
  try {
    localStorage.setItem(STORAGE_ACTIVE_TAB, newTab)
  } catch (err) {
    console.error('Save active tab error:', err)
  }
})

const accounts = ref([])
const transactions = ref([])
const isSyncing = ref(false)

const rawSyncStatusMsg = ref('ГОТОВ')
const countdownSeconds = ref(0)
const errorMsg = ref('')

let pollTimer = null
let countdownTimer = null

const formattedStatusMsg = computed(() => {
  if (countdownSeconds.value > 0) {
    return rawSyncStatusMsg.value.replace(/\d+\s*С/i, `${countdownSeconds.value}С`)
  }
  return rawSyncStatusMsg.value
})

const personsCount = computed(() => {
  const set = new Set()
  transactions.value.forEach(t => {
    if (!isP2PTransaction(t)) return
    const name = cleanPersonName(t.counter_name || t.description)
    if (name) set.add(name.toLowerCase())
  })
  return set.size
})

const fetchStatus = async () => {
  try {
    const res = await fetch(`${API_BASE}/sync/status`)
    if (res.ok) {
      const data = await res.json()
      isSyncing.value = data.running
      const newStatus = data.status || 'ГОТОВ'

      const match = newStatus.match(/(\d+)\s*С/i)
      const newSec = match ? parseInt(match[1], 10) : 0

      if (newStatus !== rawSyncStatusMsg.value) {
        rawSyncStatusMsg.value = newStatus
        if (newSec > 0 && (countdownSeconds.value === 0 || Math.abs(newSec - countdownSeconds.value) > 4)) {
          countdownSeconds.value = newSec
        }
      }
    }
  } catch (err) {
    console.error('Status fetch error:', err)
  }
}

const fetchData = async () => {
  try {
    const [accRes, txRes] = await Promise.all([
      fetch(`${API_BASE}/accounts`),
      fetch(`${API_BASE}/transactions`)
    ])
    if (accRes.ok) accounts.value = (await accRes.json()) || []
    if (txRes.ok) transactions.value = (await txRes.json()) || []
    await fetchStatus()
  } catch (err) {
    errorMsg.value = err.message
  }
}

const startFullSync = async () => {
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

  pollTimer = setInterval(() => {
    fetchStatus()
    fetchData()
  }, 3000)

  countdownTimer = setInterval(() => {
    if (countdownSeconds.value > 0) {
      countdownSeconds.value--
    }
  }, 1000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  if (countdownTimer) clearInterval(countdownTimer)
})
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 font-mono flex flex-col p-6 select-none">
    <div class="max-w-6xl w-full mx-auto space-y-8">
      <AppHeader
        v-model:activeTab="activeTab"
        :is-syncing="isSyncing"
        :sync-status-msg="formattedStatusMsg"
        :persons-count="personsCount"
        @sync="startFullSync"
      />

      <div v-if="errorMsg" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">
        ERROR: {{ errorMsg }}
      </div>

      <SmetaView v-if="activeTab === 'dashboard'" :accounts="accounts" :transactions="transactions" />
      <OperationalView v-if="activeTab === 'accounts'" :accounts="accounts" />
      <PersonsView v-if="activeTab === 'entities'" :transactions="transactions" :accounts="accounts" @refresh="fetchData" />
    </div>
  </div>
</template>
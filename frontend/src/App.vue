<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import AppHeader from './components/AppHeader.vue'
import SmetaView from './views/SmetaView.vue'
import OperationalView from './views/OperationalView.vue'
import PersonsView from './views/PersonsView.vue'
import { cleanPersonName, isP2PTransaction } from './utils/formatters.js'

const API_BASE = 'http://localhost:8080/api'

const activeTab = ref('dashboard')
const accounts = ref([])
const transactions = ref([])
const isSyncing = ref(false)
const syncStatusMsg = ref('IDLE')
const errorMsg = ref('')

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

const fetchStatus = async () => {
  try {
    const res = await fetch(`${API_BASE}/sync/status`)
    if (res.ok) {
      const data = await res.json()
      isSyncing.value = data.running
      syncStatusMsg.value = data.status
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
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 font-mono flex flex-col p-6 select-none">
    <div class="max-w-6xl w-full mx-auto space-y-8">
      <AppHeader
        v-model:activeTab="activeTab"
        :is-syncing="isSyncing"
        :sync-status-msg="syncStatusMsg"
        :persons-count="personsCount"
        @sync="startFullSync"
      />

      <div v-if="errorMsg" class="bg-rose-500/10 border border-rose-500/30 text-rose-400 p-3 rounded text-xs">
        ERROR: {{ errorMsg }}
      </div>

      <SmetaView v-if="activeTab === 'dashboard'" :accounts="accounts" :transactions="transactions" />
      <OperationalView v-if="activeTab === 'accounts'" :accounts="accounts" />
      <PersonsView v-if="activeTab === 'entities'" :transactions="transactions" />
    </div>
  </div>
</template>
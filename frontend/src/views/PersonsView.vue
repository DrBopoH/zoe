<script setup>
import { ref, computed } from 'vue'
import { formatMoney, cleanPersonName, isP2PTransaction } from '../utils/formatters.js'

const props = defineProps({
  transactions: { type: Array, default: () => [] }
})

const searchPersonQuery = ref('')

const uniquePersons = computed(() => {
  const personsMap = new Map()

  props.transactions.forEach(t => {
    if (!isP2PTransaction(t)) return

    const rawName = t.counter_name || t.description
    const name = cleanPersonName(rawName)
    if (!name) return

    const key = name.toLowerCase()

    if (!personsMap.has(key)) {
      personsMap.set(key, {
        name: name,
        inflow: 0,
        outflow: 0,
        txCount: 0,
        lastTxTime: t.time
      })
    }

    const p = personsMap.get(key)
    p.txCount++

    if (t.amount > 0) {
      p.inflow += t.amount
    } else {
      p.outflow += Math.abs(t.amount)
    }

    if (new Date(t.time) > new Date(p.lastTxTime)) {
      p.lastTxTime = t.time
    }
  })

  let list = Array.from(personsMap.values()).map(p => ({
    ...p,
    delta: p.inflow - p.outflow
  }))

  if (searchPersonQuery.value.trim()) {
    const q = searchPersonQuery.value.toLowerCase().trim()
    list = list.filter(p => p.name.toLowerCase().includes(q))
  }

  return list.sort((a, b) => (b.inflow + b.outflow) - (a.inflow + a.outflow))
})

defineExpose({
  count: computed(() => uniquePersons.value.length)
})
</script>

<template>
  <main class="space-y-4">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-3">
      <div class="text-xs font-bold text-slate-400 uppercase tracking-wider">
        ЛИЦА
      </div>

      <div class="w-full sm:w-64">
        <input
          v-model="searchPersonQuery"
          type="text"
          placeholder="Поиск..."
          class="w-full bg-slate-900 border border-slate-800 rounded px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500 font-mono"
        />
      </div>
    </div>

    <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
            <th class="p-3">ИМЯ</th>
            <th class="p-3 text-center">ТРАНЗАКЦИЙ</th>
            <th class="p-3 text-right">ВХОДЯЩИЕ</th>
            <th class="p-3 text-right">ИСХОДЯЩИЕ</th>
            <th class="p-3 text-right">ИТОГ</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono">
          <tr
            v-for="person in uniquePersons"
            :key="person.name"
            class="hover:bg-slate-800/40 transition-colors"
          >
            <td class="p-3 font-bold text-slate-200">
              {{ person.name }}
            </td>

            <td class="p-3 text-center text-slate-400 font-bold">
              {{ person.txCount }}
            </td>

            <td class="p-3 text-right text-emerald-400 font-bold">
              {{ person.inflow > 0 ? '+' + formatMoney(person.inflow) + ' ₴' : '—' }}
            </td>

            <td class="p-3 text-right text-rose-400 font-bold">
              {{ person.outflow > 0 ? '-' + formatMoney(person.outflow) + ' ₴' : '—' }}
            </td>

            <td class="p-3 text-right font-bold">
              <span :class="person.delta >= 0 ? 'text-emerald-400' : 'text-rose-400'">
                {{ person.delta >= 0 ? '+' : '' }}{{ formatMoney(person.delta) }} ₴
              </span>
            </td>
          </tr>

          <tr v-if="uniquePersons.length === 0">
            <td colspan="5" class="p-8 text-center text-slate-600 text-xs uppercase">
              Список пуст
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </main>
</template>
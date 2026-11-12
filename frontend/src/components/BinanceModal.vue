<script setup>
import { ref, computed, watch, inject, onBeforeUnmount } from 'vue'
import { apiGetJson, apiSend } from '../utils/api.js'
import { formatMoney } from '../utils/formatters.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'

// Портфель Binance: оценка в ₴ по последнему снимку. Снимок берёт бэкенд (read-only ключ из .env), здесь только просмотр и «Обновить».
const props = defineProps({
  account: { type: Object, default: null },
  isOpen: { type: Boolean, default: false }
})
const emit = defineEmits(['close'])
const refresh = inject('refreshData', null)

const info = ref({ configured: true, snapshot: null, days: 0, error: '', history: null })
const busy = ref(false)
const error = ref('')

const detail = computed(() => info.value.snapshot?.detail || {})
const items = computed(() => [...(detail.value.items || [])].sort((a, b) => b.value_uah - a.value_uah))
const unpriced = computed(() => detail.value.unpriced || [])
const warnings = computed(() => detail.value.warnings || [])
const total = computed(() => info.value.snapshot?.total ?? 0)

// История по минутам: идёт ли восстановление, чем закончилось прошлое
const hist = computed(() => info.value.history?.status || {})
const histRange = computed(() => {
  const h = info.value.history
  if (!h || !h.first) return ''
  const f = (u) => new Date(u * 1000).toLocaleDateString('ru-RU')
  return `${f(h.first)} — ${f(h.last)} · ${h.points.toLocaleString('ru-RU')} точек`
})
const residual = computed(() => Object.entries(hist.value.residual || {}).map(([a, v]) => `${a} ${v > 0 ? '+' : ''}${Number(v.toFixed(8))}`))
const pct = computed(() => (hist.value.total ? Math.min(100, Math.round((hist.value.done / hist.value.total) * 100)) : 0))
let timer = 0
const startHistory = async () => {
  try {
    await apiSend('POST', '/binance/history')
    error.value = ''
  } catch (e) {
    error.value = String(e.message || e)
    return
  }
  await load()
}
const num = (v) => (v ? Number(v.toFixed(8)).toString() : '')
const when = computed(() => {
  const t = info.value.snapshot?.taken_at
  return t ? new Date(t * 1000).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—'
})

const load = async () => {
  try {
    info.value = await apiGetJson('/binance')
    error.value = ''
  } catch (e) {
    error.value = String(e.message || e)
  }
}
const update = async () => {
  busy.value = true
  try {
    info.value = await (await apiSend('POST', '/binance')).json()
    error.value = ''
    if (refresh) await refresh()
  } catch (e) {
    error.value = String(e.message || e)
  } finally {
    busy.value = false
  }
}
watch(() => props.isOpen, (v) => { if (v) load() }, { immediate: true })
// пока идёт восстановление — опрашиваем ход; по завершении подтягиваем свежие данные
watch([() => props.isOpen, () => hist.value.running], ([open, running]) => {
  clearInterval(timer)
  if (open && running) {
    timer = setInterval(async () => {
      await load()
      if (!hist.value.running && refresh) refresh()
    }, 1500)
  }
}, { immediate: true })
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-3xl" @close="emit('close')">
    <div class="p-5 space-y-4 font-mono max-h-[85vh] overflow-y-auto">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-xs font-bold uppercase tracking-wider text-slate-300">Портфель · Binance</div>
          <div class="text-[10px] text-slate-500">
            снимок {{ when }}<template v-if="info.snapshot?.usdt_uah"> · USDT = {{ formatMoney(info.snapshot.usdt_uah) }} ₴</template> · дней в истории: {{ info.days }}
          </div>
        </div>
        <div class="text-right shrink-0">
          <div class="text-[10px] uppercase text-slate-500">Оценка</div>
          <div class="text-lg font-bold text-yellow-300">{{ formatMoney(total) }} ₴</div>
        </div>
      </div>

      <div v-if="!info.configured" class="border border-amber-500/30 bg-amber-500/5 rounded p-3 text-[11px] text-amber-200 space-y-1">
        <div>Ключ не задан. В <span class="font-bold">.env</span> добавь:</div>
        <div class="text-slate-300">BINANCE_API_KEY=…<br>BINANCE_API_SECRET=…</div>
        <div class="text-slate-500">Ключ только на чтение (Enable Reading), без торговли и вывода. Потом перезапусти бэкенд.</div>
      </div>

      <div v-if="error || info.error" class="border border-rose-500/30 bg-rose-500/5 rounded p-3 text-[11px] text-rose-300 break-words">{{ error || info.error }}</div>
      <div v-for="w in warnings" :key="w" class="text-[11px] text-amber-300">⚠ {{ w }}</div>
      <div v-if="unpriced.length" class="text-[11px] text-amber-300">Нет цены (в сумму не входят): {{ unpriced.join(', ') }}</div>

      <div v-if="items.length" class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
              <th class="p-2">Монета</th><th class="p-2 text-right">Количество</th><th class="p-2 text-right">Цена</th><th class="p-2 text-right">Стоимость</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="it in items" :key="it.asset" class="border-b border-slate-800/60">
              <td class="p-2">
                <div class="text-slate-200 font-bold">{{ it.asset }}</div>
                <div class="text-[10px] text-slate-500">
                  <span v-if="it.spot">Spot {{ num(it.spot) }}</span><span v-if="it.funding"> · Funding {{ num(it.funding) }}</span><span v-if="it.earn"> · Earn {{ num(it.earn) }}</span>
                </div>
              </td>
              <td class="p-2 text-right text-slate-300">{{ num(it.amount) }}</td>
              <td class="p-2 text-right text-slate-400">{{ formatMoney(it.price_uah) }} ₴</td>
              <td class="p-2 text-right font-bold text-yellow-200">{{ formatMoney(it.value_uah) }} ₴</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="info.configured" class="text-[11px] text-slate-500">Снимков пока нет — нажми «Обновить».</div>

      <!-- история по минутам: восстанавливается из депозитов, выводов, сделок, конвертаций, P2P и покупок картой -->
      <div class="border border-slate-800 rounded p-3 space-y-2 text-[11px]">
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-wider text-slate-400 font-bold">История по минутам</div>
            <div v-if="hist.running" class="text-slate-400">{{ hist.stage }}<template v-if="hist.total"> · {{ hist.done }}/{{ hist.total }} ({{ pct }}%)</template></div>
            <div v-else-if="histRange" class="text-slate-500">{{ histRange }}<template v-if="hist.flows"> · движений: {{ hist.flows }}</template></div>
            <div v-else class="text-slate-500">ещё не восстановлена</div>
          </div>
          <UiButton variant="outline" size="sm" :disabled="hist.running || !info.configured" @click="startHistory">{{ hist.running ? '…' : histRange ? 'Пересчитать' : 'Восстановить' }}</UiButton>
        </div>
        <div v-if="hist.running" class="h-1 bg-slate-800 rounded overflow-hidden"><div class="h-full bg-yellow-400 transition-all" :style="{ width: pct + '%' }"></div></div>
        <div v-if="hist.error" class="text-rose-300 break-words">{{ hist.error }}</div>
        <div v-if="residual.length" class="text-amber-300">Не объяснено историей (проценты Earn, пыль и т.п.), появляется скачком сегодня: {{ residual.join(', ') }}</div>
        <div v-for="w in hist.warnings || []" :key="w" class="text-amber-300 break-words">⚠ {{ w }}</div>
        <div class="text-slate-600">Приблизительно: цены — минутные свечи монет к USDT и USDT к ₴, остатки — откат известных движений от текущих.</div>
      </div>

      <div class="flex items-center justify-between gap-3">
        <span class="text-[10px] text-slate-600">Оценка в ₴ по курсу на момент снимка.</span>
        <div class="flex gap-2 shrink-0">
          <UiButton variant="outline" size="sm" :disabled="busy || !info.configured" @click="update">{{ busy ? '…' : 'Обновить' }}</UiButton>
          <UiButton variant="subtle" size="sm" @click="emit('close')">Закрыть</UiButton>
        </div>
      </div>
    </div>
  </UiModal>
</template>

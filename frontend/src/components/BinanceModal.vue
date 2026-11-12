<script setup>
import { ref, computed, watch } from 'vue'
import { apiGetJson } from '../utils/api.js'
import { formatMoney } from '../utils/formatters.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'

// Портфель Binance — только просмотр кеша. Данные приносит «Разведать» (как у банков), здесь ничего не опрашивается.
const props = defineProps({
  account: { type: Object, default: null },
  isOpen: { type: Boolean, default: false }
})
const emit = defineEmits(['close'])

const info = ref({ configured: true, detail: null, first: 0, last: 0, points: 0 })
const error = ref('')

const detail = computed(() => info.value.detail || {})
const items = computed(() => [...(detail.value.items || [])].sort((a, b) => b.value_uah - a.value_uah))
const unpriced = computed(() => detail.value.unpriced || [])
const warnings = computed(() => detail.value.warnings || [])
const total = computed(() => detail.value.total ?? props.account?.balance ?? 0)
const hist = computed(() => detail.value.history || {})
const histRange = computed(() => {
  const i = info.value
  if (!i.first) return ''
  const f = (u) => new Date(u * 1000).toLocaleDateString('ru-RU')
  return `${f(i.first)} — ${f(i.last)} · ${i.points.toLocaleString('ru-RU')} точек`
})
const residual = computed(() => Object.entries(hist.value.residual || {}).map(([a, v]) => `${a} ${v > 0 ? '+' : ''}${Number(v.toFixed(8))}`))
const num = (v) => (v ? Number(v.toFixed(8)).toString() : '')
const when = computed(() => {
  const t = detail.value.taken_at
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
watch(() => props.isOpen, (v) => { if (v) load() }, { immediate: true })
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-3xl" @close="emit('close')">
    <div class="p-5 space-y-4 font-mono max-h-[85vh] overflow-y-auto">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-xs font-bold uppercase tracking-wider text-slate-300">Портфель · Binance</div>
          <div class="text-[10px] text-slate-500">
            обновлено {{ when }}<template v-if="detail.usdt_uah"> · USDT = {{ formatMoney(detail.usdt_uah) }} ₴</template>
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

      <div v-if="error" class="border border-rose-500/30 bg-rose-500/5 rounded p-3 text-[11px] text-rose-300 break-words">{{ error }}</div>
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
      <div v-else-if="info.configured" class="text-[11px] text-slate-500">Данных пока нет — нажми «Разведать».</div>

      <!-- история по минутам: восстанавливается при «Разведать» из депозитов, выводов, сделок, конвертаций, P2P и покупок картой -->
      <div class="border border-slate-800 rounded p-3 space-y-2 text-[11px]">
        <div class="text-[10px] uppercase tracking-wider text-slate-400 font-bold">История по минутам</div>
        <div v-if="histRange" class="text-slate-500">{{ histRange }}<template v-if="hist.flows"> · движений: {{ hist.flows }}</template></div>
        <div v-else class="text-slate-500">ещё не восстановлена — появится после «Разведать»</div>
        <div v-if="residual.length" class="text-amber-300">Не объяснено историей (проценты Earn, пыль и т.п.), появляется скачком сегодня: {{ residual.join(', ') }}</div>
        <div class="text-slate-600">Приблизительно: цены — минутные свечи монет к USDT и USDT к ₴, остатки — откат известных движений от текущих. Пересобрать с нуля — вайп кеша и «Разведать».</div>
      </div>

      <div class="flex items-center justify-between gap-3">
        <span class="text-[10px] text-slate-600">Оценка в ₴ по курсу на момент последнего «Разведать».</span>
        <UiButton variant="subtle" size="sm" class="shrink-0" @click="emit('close')">Закрыть</UiButton>
      </div>
    </div>
  </UiModal>
</template>

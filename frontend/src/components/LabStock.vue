<script setup>
import { computed, onMounted, watch } from 'vue'
import UiEmpty from './ui/UiEmpty.vue'
import UiLoadError from './ui/UiLoadError.vue'
import { useWorkspaceDocs } from '../composables/useWorkspaceDocs.js'
import { CARD_KIND, stockModel, cardPerGram } from '../utils/labLedger.js'
import { formatMoney } from '../utils/formatters.js'
import { numText, dmy } from '../utils/numfmt.js'

// Склад ХимЛаба: вещества и их карточки (конкретные закупки со своей ценой) с запасом от номинала до расчётного.
// Карточки вносятся в «Учёте» под операциями «Расход»; расход со склада придёт вместе с партиями (used).
const props = defineProps({
  workspaceId: { type: String, required: true },
  used: { type: Object, default: () => new Map() } // Map(id карточки → израсходованные граммы)
})
const { docs, error, load } = useWorkspaceDocs(() => props.workspaceId, CARD_KIND)
onMounted(load)
watch(() => props.workspaceId, load)

const model = computed(() => stockModel(docs.value, props.used))
const grams = (g) => `${numText(g, 1)} г`
const barCls = (p) => (p > 0.5 ? 'bg-emerald-400/80' : p > 0.2 ? 'bg-amber-400/80' : 'bg-rose-400/80')
</script>

<template>
  <div class="space-y-4 font-mono select-none" data-testid="lab-stock">
    <UiLoadError v-if="error" :message="error" @retry="load" />

    <div class="flex flex-wrap gap-2" data-testid="stock-totals">
      <div class="px-3 py-2 rounded border border-slate-800 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-slate-400">Веществ</div>
        <div class="text-sm font-bold tabular-nums text-slate-200">{{ model.groups.length }}</div>
        <div class="text-[10px] text-slate-500">{{ model.cards }} карточ.</div>
      </div>
      <div class="px-3 py-2 rounded border border-slate-800 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-slate-400">Закуплено</div>
        <div class="text-sm font-bold tabular-nums text-slate-200">{{ formatMoney(model.valueTotal) }}</div>
      </div>
      <div class="px-3 py-2 rounded border border-emerald-500/30 bg-slate-900/60 min-w-[8.5rem]">
        <div class="text-[10px] uppercase tracking-wider font-bold text-emerald-300">На складе</div>
        <div class="text-sm font-bold tabular-nums text-emerald-400">{{ formatMoney(model.valueLeft) }}</div>
      </div>
    </div>

    <UiEmpty v-if="!model.groups.length">Карточек пока нет. Раскрой в «Учёте» операцию с бейджем «Расход» и нажми «+ Товар»: карточка попадёт на склад.</UiEmpty>

    <section v-for="g in model.groups" :key="g.key" class="rounded border border-slate-800 bg-slate-900/60" data-testid="stock-group">
      <header class="flex flex-wrap items-baseline justify-between gap-x-4 px-3 py-2 border-b border-slate-800">
        <div class="text-sm font-bold text-slate-200">{{ g.name }}</div>
        <div class="text-[11px] text-slate-400 tabular-nums">
          <span class="text-slate-200 font-bold">{{ grams(g.left) }}</span> из {{ grams(g.nominal) }}
          · <span class="text-emerald-400 font-bold">{{ formatMoney(g.valueLeft) }}</span> из {{ formatMoney(g.valueTotal) }}
        </div>
      </header>
      <ul>
        <li v-for="r in g.rows" :key="r.card.id" class="px-3 py-2 border-b border-slate-800/60 last:border-b-0 space-y-1.5" data-testid="stock-card">
          <div class="flex flex-wrap items-baseline justify-between gap-x-4 text-[11px]">
            <div class="text-slate-300">
              {{ r.card.maker || 'без производителя' }} · {{ numText(r.card.net_g, 1) }} г
              <span class="text-slate-500">· {{ r.card.qty }} × {{ formatMoney(r.card.price) }}</span>
              <span v-if="r.card.note" class="text-slate-500"> · {{ r.card.note }}</span>
            </div>
            <div class="text-slate-500 tabular-nums">{{ dmy(r.card.day) }} · {{ numText(cardPerGram(r.card), 4) }} ₴/г</div>
          </div>
          <div class="flex items-center gap-3">
            <div class="flex-1 h-1.5 rounded bg-slate-800 overflow-hidden" role="img" :aria-label="`Осталось ${Math.round(r.pct * 100)} %`">
              <div class="h-full rounded" :class="barCls(r.pct)" :style="{ width: `${Math.round(r.pct * 100)}%` }" data-testid="stock-bar" />
            </div>
            <div class="text-[11px] text-slate-400 tabular-nums whitespace-nowrap">{{ grams(r.left) }} из {{ grams(r.nominal) }} · {{ formatMoney(r.valueLeft) }}</div>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

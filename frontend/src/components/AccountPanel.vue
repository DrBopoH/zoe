<script setup>
import { formatMoney, lifeSpan } from '../utils/formatters.js'
import UiDot from './ui/UiDot.vue'

// Карта-панель «Операционной»: IBAN, карты счёта, банк, баланс. Один отрисовщик для операционки и барабана.
// card — результат prepareAccount()/cardFromTile() (utils/accountView.js)
const props = defineProps({
  card: { type: Object, required: true },
  selectable: { type: Boolean, default: false }, // рисовать отметку выбора
  selected: { type: Boolean, default: false },
  compact: { type: Boolean, default: false }, // плитка поменьше (боковые панели)
  removablePans: { type: Boolean, default: false }, // × у каждой карты (редактор алиаса)
  cached: { type: Boolean, default: true } // зелёная точка: счёт есть в кэше
})
const emit = defineEmits(['remove-pan'])
</script>

<template>
  <div
    class="ui-select-card border rounded-lg flex flex-col justify-between transition-all duration-300 shadow-lg relative overflow-hidden"
    
    :class="[compact ? 'p-3' : 'p-5', card.theme.panel, selected && 'ui-select-card--on', card.is_closed && 'ui-closed']"
  >
    <!-- Шапка: Имя IBAN + Бейдж банка -->
    <div class="flex items-start justify-between gap-3 z-10">
      <div class="space-y-1 min-w-0">
        <div class="flex items-center gap-2 min-w-0">
          <UiDot :cached="cached" />
          <div
            class="truncate"
            :class="card.displayIban === 'КАРТА' ? 'text-sm font-bold tracking-wide opacity-50 select-none' : 'text-sm font-bold tracking-wide'"
          >
            {{ card.displayIban }}
          </div>
        </div>

        <!-- Привязанная карта (PAN) -->
        <div
          v-for="(pan, i) in card.pans"
          :key="pan + i"
          class="flex items-center gap-1.5 text-[11px] pl-4"
          :class="i === 0 ? 'opacity-80' : 'opacity-50'"
        >
          <span class="opacity-50 select-none">└─</span>
          <span class="tracking-wider">{{ pan }}</span>
          <button v-if="removablePans" type="button" class="pan-x" title="Отвязать карту" @click.stop="emit('remove-pan', i)">×</button>
        </div>
      </div>

      <div class="flex flex-col items-end gap-1 shrink-0">
        <span
          v-if="card.theme.tag"
          class="text-[10px] font-bold px-2 py-0.5 rounded border uppercase font-mono"
          :class="card.theme.tagStyle"
        >
          {{ card.theme.tag }}
        </span>
        <span v-if="card.is_closed" class="ui-closed-tag">Закрыта</span>
      </div>
    </div>

    <!-- Нижний блок: Баланс -->
    <div class="space-y-1 z-10" :class="compact ? 'mt-3' : 'mt-6'">
      <span class="text-[10px] opacity-60 uppercase block font-bold tracking-wider">Баланс счета:</span>
      <span class="font-bold tracking-wide" :class="compact ? 'text-base' : 'text-xl'">
        {{ card.balance == null ? '—' : formatMoney(card.balance) + ' ₴' }}
      </span>
      <span v-if="lifeSpan(card)" class="text-[10px] opacity-60 block tracking-wider">{{ lifeSpan(card) }}</span>
    </div>

    <!-- Отметка выбора -->
    <span v-if="selectable" class="ui-select-mark" :class="selected && 'ui-select-mark--on'">
      <svg v-if="selected" viewBox="0 0 24 24" class="w-3 h-3" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12l5 5L20 7" /></svg>
    </span>
  </div>
</template>

<style scoped>
.pan-x { margin-left: 4px; line-height: 1; padding: 0 4px; border-radius: 4px; opacity: 0.5; }
.pan-x:hover { opacity: 1; background: rgb(244 63 94 / 0.3); color: rgb(253 164 175); }
</style>

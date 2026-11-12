<script setup>
import { lifeSpan, money, signed, formatMoney } from '../utils/formatters.js'
import UiDot from './ui/UiDot.vue'
import MaskedId from './ui/MaskedId.vue'
import { computed } from 'vue'
import { walletName, walletText, fmtQty, walletsOf, bondsOf } from '../utils/portfolioView.js'

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

const bonds = computed(() => bondsOf(props.card))
const wallets = computed(() => walletsOf(props.card))
// курс USDT/₴ из последней «Разведки»: Binance — международная биржа, суммы в долларах
const rate = computed(() => (props.card.portfolio && props.card.portfolio.usdt_uah) || 0)
const val = (uah) => (rate.value ? money(uah / rate.value, '$') : money(uah))
const dlt = (uah) => (rate.value ? signed(uah / rate.value) : signed(uah))
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
            class="min-w-0 text-sm font-bold tracking-wide"
            :class="card.displayIban === 'КАРТА' && 'truncate opacity-50 select-none'"
          >
            <MaskedId v-if="card.rawIban" :id="card.rawIban" interactive />
            <template v-else>{{ card.displayIban }}</template>
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
          <MaskedId v-if="card.rawPans && card.rawPans[i]" class="tracking-wider" :id="card.rawPans[i]" interactive />
          <span v-else class="tracking-wider">{{ pan }}</span>
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

    <!-- Нижний блок: слева баланс, справа в углу состав счёта.
         Брокерский: кэш, в скобках стоимость облигаций; справа ШТ · ISIN и накопленный купон.
         Binance: баланс в долларах, в скобках ₴-эквивалент; ниже дерево кошелёк → позиции.
         Валюта перед числом, у вложенных/дочерних чисел не дублируется; результат (+/−) — мелкий нижний индекс. -->
    <div class="z-10 flex items-end justify-between gap-3" :class="[compact ? 'mt-3' : 'mt-6', selectable && (bonds || wallets.length) && 'pb-8']">
      <div class="space-y-1 min-w-0">
        <span class="text-[10px] opacity-60 uppercase block font-bold tracking-wider">Баланс счета:</span>
        <span class="font-bold tracking-wide" :class="compact ? 'text-base' : 'text-xl'">
          <template v-if="bonds"><span class="whitespace-nowrap">{{ money(bonds.cash) }}</span><span class="ml-2 text-[11px] font-normal text-slate-400 opacity-80 whitespace-nowrap" title="Стоимость всех облигаций">({{ formatMoney(bonds.invested) }})</span></template>
          <template v-else-if="rate && card.balance != null"><span class="whitespace-nowrap">{{ money(card.balance / rate, '$') }}</span><span class="ml-2 text-[11px] font-normal text-slate-400 opacity-80 whitespace-nowrap">({{ money(card.balance) }})</span></template>
          <template v-else>{{ card.balance == null ? '—' : money(card.balance) }}</template>
        </span>
        <span v-if="lifeSpan(card)" class="text-[10px] opacity-60 block tracking-wider">{{ lifeSpan(card) }}</span>
      </div>

      <!-- Облигации: ШТ · ISIN и накопленный купон -->
      <div v-if="bonds && bonds.bonds && bonds.bonds.length" class="text-right font-mono text-[10px] leading-4 space-y-1 shrink-0">
        <div v-for="b in bonds.bonds" :key="b.isin" :title="(b.name || b.isin) + ' — накопленный купон (НКД) на дату, оценка по ставке и графику'">
          <div class="opacity-80 whitespace-nowrap">{{ b.qty }} шт · {{ b.isin }}</div>
          <div class="text-emerald-300 whitespace-nowrap">{{ money(b.accrued) }}</div>
        </div>
      </div>

    </div>

    <!-- Binance: кошелёк → позиции. Число в скобках: основное значение в $, результат — мелкий нижний индекс (как степень, только снизу), без повтора символа -->
    <div v-if="wallets.length" class="z-10 mt-3 space-y-1.5 font-mono text-[11px]" :class="selectable && 'pb-6'">
      <div v-for="w in wallets" :key="w.wallet">
        <div class="font-bold tracking-wider text-[10px]" :class="walletText(w.wallet)">{{ walletName(w.wallet) }}</div>
        <div v-for="it in w.items" :key="it.asset" class="pl-3 flex items-baseline gap-1.5">
          <span class="opacity-50 select-none">└</span>
          <span class="font-bold">{{ it.asset }}</span>
          <span>{{ fmtQty(it.asset, it.qty) }}</span>
          <span v-if="it.priced" class="text-[10px] text-slate-400 opacity-90">({{ val(it.value_uah) }}<sub v-if="it.delta != null" class="sub-d" :class="it.delta >= 0 ? 'text-emerald-400' : 'text-rose-400'" :title="it.delta_kind === 'last' ? 'Последнее начисление' : 'Результат по последним покупкам'">{{ dlt(it.delta) }}</sub>)</span>
        </div>
      </div>
    </div>

    <!-- Отметка выбора -->
    <span v-if="selectable" class="ui-select-mark" :class="selected && 'ui-select-mark--on'">
      <svg v-if="selected" viewBox="0 0 24 24" class="w-3 h-3" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12l5 5L20 7" /></svg>
    </span>
  </div>
</template>

<style scoped>
.sub-d { font-size: 9px; vertical-align: baseline; position: relative; top: 0.35em; margin-left: 2px; line-height: 0; }
.pan-x { margin-left: 4px; line-height: 1; padding: 0 4px; border-radius: 4px; opacity: 0.5; }
.pan-x:hover { opacity: 1; background: rgb(244 63 94 / 0.3); color: rgb(253 164 175); }
</style>

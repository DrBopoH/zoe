<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import UiButton from '../components/ui/UiButton.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, signedPct, drawdown, athInfo, changeSince, fearGreedLevel, unitText } from '../utils/macro.js'

// Экран «Крипто»: недельные цены Bitcoin, Ethereum, BNB, Solana (публичные свечи Binance), просадка от максимума и индекс страха и жадности.
// Не прогноз и не совет: картина того, где рынок относительно своих пиков и настроения.
const props = defineProps({
  crypto: { type: Object, default: () => ({}) }, // crypto_btc, crypto_eth, crypto_bnb, crypto_sol, crypto_fng
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'crypto')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся цены Binance и индекс страха и жадности'

// Просадку считаем на месте, из цен: отдельные ряды в кеш не пишем
const withDd = computed(() => ({ ...props.crypto, crypto_btc_dd: drawdown(props.crypto.crypto_btc), crypto_eth_dd: drawdown(props.crypto.crypto_eth) }))
const btc = computed(() => lastPoint(props.crypto.crypto_btc))
const ath = computed(() => athInfo(props.crypto.crypto_btc))
const y1 = computed(() => changeSince(props.crypto.crypto_btc, props.today, 1))
const fng = computed(() => lastPoint(props.crypto.crypto_fng))
const FNG_TEXT = { 'extreme-fear': 'крайний страх', fear: 'страх', neutral: 'нейтрально', greed: 'жадность', 'extreme-greed': 'крайняя жадность' }
const fngWord = computed(() => FNG_TEXT[fearGreedLevel(fng.value?.v)] || '')
// Цвет числа в шапке — как у линии: страх голубой, середина серая, жадность янтарная
const fngColor = computed(() => ({ 'extreme-fear': 'text-sky-300', fear: 'text-sky-300', neutral: 'text-slate-300', greed: 'text-amber-300', 'extreme-greed': 'text-amber-300' })[fearGreedLevel(fng.value?.v)] || 'text-slate-300')
const dateOf = (iso) => (iso ? iso.split('-').reverse().join('.') : '')
const coin = (id) => lastPoint(props.crypto[id])
const has = computed(() => Object.values(props.crypto).some((a) => a && a.length))
</script>

<template>
  <div class="space-y-4">
    <section class="flex flex-wrap items-center gap-3 text-xs">
      <h2 class="font-bold text-slate-300 uppercase tracking-wider mr-2">Крипто</h2>
      <span v-if="btc" class="text-slate-500">цены по закрытию недели · последняя точка {{ dateOf(btc.d) }}</span>
      <UiButton variant="secondary" size="sm" class="ml-auto" :disabled="running" @click="refresh">
        <template v-if="running">ОБНОВЛЯЮ...</template>
        <template v-else>ОБНОВИТЬ</template>
      </UiButton>
    </section>

    <MacroPanel title="Bitcoin, $" unit="$" :series="crypto" :today="today" :ids="['crypto_btc']" log :empty="EMPTY">
      <template #summary>
        <span v-if="btc" class="text-slate-400">Сейчас <b class="text-orange-300 tabular-nums">{{ unitText(btc.v, '$') }}</b></span>
        <span v-if="ath" class="text-slate-400" :title="`Максимум по закрытию недели: ${unitText(ath.ath, '$')}, ${dateOf(ath.date)}`">От максимума <b class="tabular-nums" :class="ath.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(ath.fromAth) }}</b></span>
        <span v-if="y1 !== null" class="text-slate-400">За год <b class="tabular-nums" :class="y1 >= 0 ? 'text-emerald-400' : 'text-rose-400'">{{ signedPct(y1) }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Ethereum, BNB, Solana, $" unit="$" :series="crypto" :today="today" :ids="['crypto_eth', 'crypto_bnb', 'crypto_sol']" log :empty="EMPTY">
      <template #summary>
        <span v-if="coin('crypto_eth')" class="text-slate-400">ETH <b class="text-violet-300 tabular-nums">{{ unitText(coin('crypto_eth').v, '$') }}</b></span>
        <span v-if="coin('crypto_bnb')" class="text-slate-400">BNB <b class="text-amber-300 tabular-nums">{{ unitText(coin('crypto_bnb').v, '$') }}</b></span>
        <span v-if="coin('crypto_sol')" class="text-slate-400">SOL <b class="text-emerald-300 tabular-nums">{{ unitText(coin('crypto_sol').v, '$') }}</b></span>
      </template>
    </MacroPanel>

    <MacroPanel title="Просадка от исторического максимума" :series="withDd" :today="today" :ids="['crypto_btc_dd', 'crypto_eth_dd']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. 0 % — цена на максимуме, −50 % — вдвое ниже пика">
      <template #summary>
        <span v-if="ath" class="text-slate-400">Bitcoin сейчас <b class="tabular-nums" :class="ath.fromAth >= -1 ? 'text-emerald-400' : 'text-rose-400'">{{ plainPct(ath.fromAth) }}</b> от пика</span>
      </template>
    </MacroPanel>

    <MacroPanel title="Индекс страха и жадности крипторынка" unit="п." :series="crypto" :today="today" :ids="['crypto_fng']" :empty="EMPTY" hint="Колесо: масштаб, перетаскивание: сдвиг. До 25 крайний страх, выше 75 крайняя жадность">
      <template #summary>
        <span v-if="fng" class="text-slate-400">Сейчас <b class="tabular-nums" :class="fngColor">{{ Math.round(fng.v) }}</b><span class="text-slate-600"> · {{ fngWord }}</span></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Цены: публичные свечи Binance (пара к USDT, неделя). Индекс страха и жадности: alternative.me. Это картина рынка, а не прогноз и не совет.</div>
  </div>
</template>

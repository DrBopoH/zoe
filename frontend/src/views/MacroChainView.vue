<script setup>
import { computed } from 'vue'
import MacroPanel from '../components/MacroPanel.vue'
import MarketHeader from '../components/MarketHeader.vue'
import { useMarketRefresh } from '../composables/useMarketRefresh.js'
import { lastPoint, plainPct, signedPct, unitText, hasPoints, dateOf } from '../utils/macro.js'
import { p2pDerived, feeRows, fundingLevel } from '../utils/chain.js'

// Экран «Крипто-контур»: всё, что стоит на пути «банк → P2P USDT → торговля → вывод»: цена USDT за гривны (P2P Binance) и её спред,
// переплата к курсу НБУ, комиссии сетей за перевод, фандинг и открытый интерес фьючерсов, выпуск стейблкоинов, доля Bitcoin.
// Цены P2P и комиссии — снимки на момент «Обновить»: история копится от обновления к обновлению. Не прогноз и не совет.
const props = defineProps({
  chain: { type: Object, default: () => ({}) }, // ряды источника «chain» и usd_uah (курс НБУ)
  today: { type: String, required: true },
  running: { type: Boolean, default: false }
})
const { refresh } = useMarketRefresh(() => 'chain')
const EMPTY = 'Нет данных. Нажми «Обновить»: подтянутся цены P2P, комиссии сетей и показатели фьючерсов'
const all = computed(() => ({ ...props.chain, ...p2pDerived(props.chain) }))
const has = computed(() => hasPoints({ ...props.chain, usd_uah: [] }))
const ask = computed(() => lastPoint(props.chain.p2p_ask))
const bid = computed(() => lastPoint(props.chain.p2p_bid))
const spread = computed(() => lastPoint(all.value.p2p_spread))
const prem = computed(() => lastPoint(all.value.p2p_prem_ask))
const fees = computed(() => feeRows(props.chain))
const fBtc = computed(() => lastPoint(props.chain.fund_btc))
const fEth = computed(() => lastPoint(props.chain.fund_eth))
const dom = computed(() => lastPoint(props.chain.dom_btc))
const mcap = computed(() => lastPoint(props.chain.mcap_total))
const usdt = computed(() => lastPoint(props.chain.stable_usdt))
const money = (v, d = 2) => (v === null ? '—' : String(v.toFixed(d)).replace('.', ','))
</script>

<template>
  <div class="space-y-4">
    <MarketHeader title="Крипто-контур" :running="running" @refresh="refresh">
      <span v-if="ask" class="text-slate-500">P2P на {{ dateOf(ask.d) }}</span>
    </MarketHeader>

    <section class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs">
      <div v-if="ask && bid" class="flex flex-wrap items-baseline gap-x-6 gap-y-1">
        <span class="text-slate-400">Купить USDT <b class="text-rose-400 tabular-nums">{{ money(ask.v) }} ₴</b></span>
        <span class="text-slate-400">Продать USDT <b class="text-emerald-400 tabular-nums">{{ money(bid.v) }} ₴</b></span>
        <span v-if="spread" class="text-slate-400" title="Разница цен покупки и продажи от середины: столько теряется на круге «купил и сразу продал»">Спред <b class="text-amber-300 tabular-nums">{{ plainPct(spread.v) }}</b></span>
        <span v-if="prem" class="text-slate-400" title="Насколько покупка USDT дороже официального курса доллара НБУ">К курсу НБУ <b class="tabular-nums" :class="prem.v > 3 ? 'text-rose-400' : 'text-slate-100'">{{ signedPct(prem.v) }}</b></span>
      </div>
      <div v-else class="text-slate-500">{{ EMPTY }}</div>
      <div class="mt-2 text-[11px] text-slate-600">Цена — медиана первых пяти объявлений, на которые хватает остатка и лимитов для 10 000 ₴. Платёжные методы разные: у конкретного банка цена другая.</div>
    </section>

    <section class="rounded-lg border border-slate-800 bg-slate-900/40 overflow-x-auto">
      <div class="px-3 py-2 text-[11px] text-slate-500">Сколько стоит перевести USDT или BTC по сети (снимок на момент «Обновить»). TRON считается как худший случай: без замороженных ресурсов, сжигание TRX.</div>
      <table class="w-full text-xs tabular-nums">
        <thead class="text-[10px] uppercase tracking-wider text-slate-500">
          <tr class="border-t border-slate-800/70"><th class="px-3 py-1.5 text-left font-bold">Сеть</th><th class="px-2 text-right font-bold">$</th><th class="px-2 text-right font-bold">₴</th><th class="px-3 text-left font-bold">Расчёт</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60">
          <tr v-for="f in fees" :key="f.id">
            <td class="px-3 py-1.5 text-slate-300">{{ f.label }}</td>
            <td class="px-2 text-right text-slate-100">{{ f.usd === null ? '—' : money(f.usd, f.usd < 0.1 ? 3 : 2) }}</td>
            <td class="px-2 text-right text-slate-100">{{ f.uah === null ? '—' : money(f.uah) }}</td>
            <td class="px-3 text-slate-500">{{ f.note }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <MacroPanel title="P2P: цена USDT в гривнах" unit="₴" :series="all" :today="today" :ids="['p2p_ask', 'p2p_bid']" :empty="EMPTY" hint="Каждое «Обновить» добавляет точку за день: история копится с первого запуска" />
    <MacroPanel title="P2P: переплата и спред, %" unit="%" :series="all" :today="today" :ids="['p2p_prem_ask', 'p2p_prem_bid', 'p2p_spread']" :empty="EMPTY" hint="Покупка и продажа USDT относительно официального курса НБУ и спред между ними" />
    <MacroPanel title="Комиссия перевода, $" unit="$" :series="all" :today="today" :ids="['fee_tron_usd', 'fee_eth_usd', 'fee_btc_usd']" :empty="EMPTY" hint="Снимки: график заполняется по мере обновлений" />
    <MacroPanel title="Комиссия Bitcoin: медиана блоков, сат/vB" unit="сат/vB" :series="all" :today="today" :ids="['fee_btc_sat']" :empty="EMPTY" hint="Среднее за сутки из медианы комиссий блоков (mempool.space), 3 месяца" />

    <MacroPanel title="Фандинг бессрочных фьючерсов, % годовых" unit="%" :series="all" :today="today" :ids="['fund_btc', 'fund_eth']" :empty="EMPTY" hint="Binance, среднее за сутки × 3 выплаты × 365. Плюс: лонги платят шортам, минус — наоборот">
      <template #summary>
        <span v-if="fBtc" class="text-slate-400">BTC <b class="text-orange-300 tabular-nums">{{ plainPct(fBtc.v) }}</b> · {{ fundingLevel(fBtc.v) }}</span>
        <span v-if="fEth" class="text-slate-400">ETH <b class="text-violet-300 tabular-nums">{{ plainPct(fEth.v) }}</b></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Открытый интерес фьючерсов, $ млрд" unit="$ млрд" :series="all" :today="today" :ids="['oi_btc', 'oi_eth']" :empty="EMPTY" hint="Binance отдаёт только последние 30 дней: дальше история копится по обновлениям" />

    <MacroPanel title="Стейблкоины в обращении, $ млрд" unit="$ млрд" :series="all" :today="today" :ids="['stable_usdt', 'stable_usdc']" :empty="EMPTY" hint="DefiLlama, неделя. Рост выпуска — приток денег на крипторынок">
      <template #summary>
        <span v-if="usdt" class="text-slate-400">USDT <b class="text-emerald-400 tabular-nums">{{ unitText(usdt.v, '$ млрд') }}</b></span>
      </template>
    </MacroPanel>
    <MacroPanel title="Доля Bitcoin и капитализация рынка" unit="%" :series="all" :today="today" :ids="['dom_btc']" :empty="EMPTY" hint="CoinGecko, снимки по обновлениям">
      <template #summary>
        <span v-if="dom" class="text-slate-400">Доля BTC <b class="text-orange-300 tabular-nums">{{ plainPct(dom.v) }}</b></span>
        <span v-if="mcap" class="text-slate-400">Рынок <b class="text-sky-300 tabular-nums">{{ unitText(mcap.v, '$ трлн') }}</b></span>
      </template>
    </MacroPanel>

    <div v-if="has" class="text-[11px] text-slate-600">Данные: P2P и фьючерсы Binance, DefiLlama, mempool.space, TronGrid, публичный RPC Ethereum, CoinGecko; курс НБУ. Это справочные цифры для расчётов, а не прогноз и не совет.</div>
  </div>
</template>

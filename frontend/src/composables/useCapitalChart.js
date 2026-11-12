import { computed, ref, watch } from 'vue'
import { apiGetJson, q } from '../utils/api.js'
import { bankTheme } from '../utils/bankTheme.js'

// Данные для CandleChart: гривневые счета, их операции, крипто-счета и выплаты по облигациям.
// `pickedIds` — ref с массивом id счетов, по которым строить график; пусто → все (Смета — всегда все, Операционная — выбранные карты).
// Операции с облигациями из обзора брокерского счёта → строки истории. Движение внутри счёта: деньги ↔ облигации, капитал не меняют.
function bondOpsOf(accountId, ov) {
  const names = new Map((ov.bonds || []).map(b => [b.isin, b.name]))
  const label = (isin, name, qty) => {
    const bond = name || names.get(isin) || isin
    return `${bond}${qty ? ` · ${qty} шт.` : ''}` // вид операции — плашкой в таблице
  }
  const out = []
  for (const r of ov.rows || []) {
    if (r.kind !== 'buy' && r.kind !== 'sell') continue
    out.push({
      id: `icuop_${r.id}`, account_id: accountId, time: new Date(r.time * 1000).toISOString(), amount: r.amount, currency_code: 980,
      memo: r.kind, isin: r.isin, qty: r.qty, description: label(r.isin, '', r.qty), comment: r.note || ''
    })
  }
  for (const e of ov.events || []) { // полученные купоны и погашения (будущие — не операции)
    if (!e.done) continue
    out.push({
      id: `icuev_${accountId}_${e.date}_${e.isin}_${e.kind}`, account_id: accountId, time: new Date(`${e.date}T12:00:00`).toISOString(),
      amount: e.amount, currency_code: 980, memo: e.kind, isin: e.isin, qty: e.qty, description: label(e.isin, e.name, e.qty)
    })
  }
  return out
}

export function useCapitalChart(accounts, transactions, pickedIds = ref([])) {
  // Выплаты по облигациям брокерских счетов (купоны и погашения) — маркеры на графике и таймлайн вперёд.
  // Счета обновляются вместе с версией данных, поэтому перечитываем при каждой новой загрузке.
  const allPayEvents = ref([])
  const icuOps = ref([]) // покупки/продажи/полученные выплаты — справочные строки истории (в потоке операций и на графике их нет)
  // Переводы между кошельками Binance — тоже справочные строки истории (капитал не меняют)
  const bnMoves = ref([])
  const bnTrades = ref([]) // покупки и продажи монет внутри кошельков
  const bnSwaps = ref([]) // обмены монет: Convert, пыль → BNB, стейкинг
  watch(accounts, async (list) => {
    const bn = (list || []).find(a => a.bank_key === 'binance')
    if (!bn) { bnTrades.value = []; return }
    try {
      const trades = await apiGetJson('/binance/trades')
      bnTrades.value = (trades || []).map(x => ({
        id: `bntr_${x.id}`, account_id: bn.id, time: new Date(x.t).toISOString(), amount: 0, currency_code: 980,
        memo: 'trade', exact: true, side: x.side, wallet: x.wallet,
        description: `${x.qty} ${x.asset} ${x.side === 'buy' ? 'за' : '→'} ${x.quote_qty} ${x.quote}`
      }))
    } catch (e) {
      console.error('Binance trades error:', e)
    }
  }, { immediate: true })

  // Обмен: «0.2 SOL → 0.177 BNSOL»; если монет несколько (пыль) — первая и «+N»
  const legs = (l) => (l.length ? `${l[0].qty} ${l[0].asset}${l.length > 1 ? ` +${l.length - 1}` : ''}` : '')
  watch(accounts, async (list) => {
    const bn = (list || []).find(a => a.bank_key === 'binance')
    if (!bn) { bnSwaps.value = []; return }
    try {
      const swaps = await apiGetJson('/binance/swaps')
      bnSwaps.value = (swaps || []).map(x => ({
        id: `bnsw_${x.id}`, account_id: bn.id, time: new Date(x.t).toISOString(), amount: 0, currency_code: 980,
        memo: 'swap', exact: true, kind: x.kind, wallet: x.wallet,
        description: `${legs(x.out)} → ${legs(x.in)}`
      }))
    } catch (e) {
      console.error('Binance swaps error:', e)
    }
  }, { immediate: true })

  watch(accounts, async (list) => {
    const bn = (list || []).find(a => a.bank_key === 'binance')
    if (!bn) { bnMoves.value = []; return }
    try {
      const moves = await apiGetJson('/binance/moves')
      // автоподписка (проценты сами легли обратно в Earn) — не перевод: её роль уже показывает начисление с бейджем EARN
      bnMoves.value = (moves || []).filter(m => !m.auto).map(m => ({
        id: `bnmv_${m.id}`, account_id: bn.id, time: new Date(m.t).toISOString(), amount: 0, currency_code: 980,
        memo: 'move', exact: true, from: m.from, to: m.to, description: `${m.qty} ${m.asset}`
      }))
    } catch (e) {
      console.error('Binance moves error:', e)
    }
  }, { immediate: true })

  watch(accounts, async (list) => {
    const manual = (list || []).filter(a => a.bank_key === 'icu')
    const ops = []
    try {
      const all = await Promise.all(manual.map(async a => {
        const ov = await apiGetJson(`/icu/ledger?account_id=${q(a.id)}`)
        const color = bankTheme(a.bank_key || 'icu', a.type).color // маркеры в цвет счёта
        ops.push(...bondOpsOf(a.id, ov))
        return (ov.events || []).map(e => ({ ...e, color, account_id: a.id }))
      }))
      allPayEvents.value = all.flat()
      icuOps.value = ops
    } catch (e) {
      console.error('ICU events error:', e)
    }
  }, { immediate: true })

  const uahAccounts = computed(() => accounts.value.filter(a => a.currency_code === 980))
  // Капитал считается по гривневым счетам, значит и динамика — только по их операциям (валютные искажали бы свечи)
  const uahTransactions = computed(() => {
    const ids = new Set(uahAccounts.value.map(a => a.id))
    return transactions.value.filter(t => ids.has(t.account_id))
  })

  const chartAccounts = computed(() => {
    if (!pickedIds.value.length) return uahAccounts.value
    const want = new Set(pickedIds.value)
    return uahAccounts.value.filter(a => want.has(a.id))
  })
  const chartIds = computed(() => new Set(chartAccounts.value.map(a => a.id)))
  const chartTotalUAH = computed(() => chartAccounts.value.reduce((sum, a) => sum + a.balance, 0))
  const chartTransactions = computed(() => uahTransactions.value.filter(t => chartIds.value.has(t.account_id)))
  const payEvents = computed(() => allPayEvents.value.filter(e => chartIds.value.has(e.account_id)))
  // Операции Binance — обычная история счёта: свечи двигаются по ним сразу, пока разведка идёт. Когда собран минутный ряд
  // оценки, скачки депозитов уже в нём, и график сам перестаёт считать эти операции второй раз (CandleChart, binanceIds).
  const binanceIdList = computed(() => chartAccounts.value.filter(a => a.bank_key === 'binance').map(a => a.id))

  const bondOps = computed(() => [...icuOps.value, ...bnMoves.value, ...bnTrades.value, ...bnSwaps.value]) // имя прежнее: справочные строки истории всех счетов

  return { bondOps, uahAccounts, uahTransactions, chartTransactions, chartTotalUAH, binanceIdList, payEvents }
}

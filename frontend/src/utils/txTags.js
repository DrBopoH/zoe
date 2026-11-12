// Бейджи операций в истории: вид движения — тегом, а не словом в описании.
//
// Схема бейджей Binance:
//   депозит / P2P / вывод / бонус — вид + кошелёк, где произошло:   [ИНВЕСТИЦИЯ] [SPOT]   [ИНВЕСТИЦИЯ] [FUNDING]
//   начисление Earn               — только [EARN]
//   перевод между кошельками      — откуда → куда:                   [SPOT] → [EARN]   [SPOT] → [STRATEGY]
//   сделка в кошельке             — [ПОКУПКА] [SPOT] / [ПРОДАЖА] [SPOT]
const STYLE = {
  invest: 'bg-blue-500/15 text-blue-300 border-blue-500/30',
  out: 'bg-orange-500/15 text-orange-300 border-orange-500/30',
  bonus: 'bg-pink-500/15 text-pink-300 border-pink-500/30',
  buy: 'bg-sky-500/15 text-sky-300 border-sky-500/30',
  sell: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
  coupon: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
  redemption: 'bg-violet-500/15 text-violet-300 border-violet-500/30',
  // кошельки Binance
  spot: 'bg-slate-500/15 text-slate-300 border-slate-500/30',
  funding: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
  strategy: 'bg-fuchsia-500/15 text-fuchsia-300 border-fuchsia-500/30',
  futures: 'bg-orange-500/15 text-orange-300 border-orange-500/30',
  earn: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'
}
const tag = (label, style) => ({ label, cls: STYLE[style] })
const ARROW = { label: '→', sep: true }

// Кошельки Binance (названия из журнала выписки)
const WALLET = {
  Spot: ['SPOT', 'spot'],
  Funding: ['FUNDING', 'funding'],
  Strategy: ['STRATEGY', 'strategy'],
  'USD-M Futures': ['FUTURES', 'futures'],
  Earn: ['EARN', 'earn']
}
const walletTag = (w) => (WALLET[w] ? tag(WALLET[w][0], WALLET[w][1]) : tag(String(w).toUpperCase(), 'spot'))

// Облигации брокерского счёта (справочные строки истории)
const MEMO = { buy: 'ПОКУПКА', sell: 'ПРОДАЖА', coupon: 'КУПОН', redemption: 'ПОГАШЕНИЕ' }

// Движения Binance (kind и wallet приходят с бэкенда). «Инвестиция» — только депозит и P2P: капитал заходит в портфель.
const KIND = {
  deposit: () => tag('ИНВЕСТИЦИЯ', 'invest'),
  p2p: () => tag('ИНВЕСТИЦИЯ', 'invest'),
  withdraw: () => tag('ВЫВОД', 'out'),
  bonus: () => tag('БОНУС', 'bonus')
}

export function tagsFor(t) {
  if (t.memo === 'trade') return [tag(t.side === 'buy' ? 'ПОКУПКА' : 'ПРОДАЖА', t.side === 'buy' ? 'buy' : 'sell'), walletTag(t.wallet)]
  if (t.memo === 'move') return [walletTag(t.from), ARROW, walletTag(t.to)]
  if (t.memo) return [tag(MEMO[t.memo] || t.memo.toUpperCase(), t.memo)]
  if (t.kind === 'dividend') return [walletTag('Earn')] // начисление: деньги пришли из Earn, кошелёк зачисления не показываем
  if (t.kind && KIND[t.kind]) return t.wallet ? [KIND[t.kind](), walletTag(t.wallet)] : [KIND[t.kind]()]
  if (t.internal) return [tag('ИНВЕСТИЦИЯ', 'invest')] // перевод на брокерский счёт/Binance и запись журнала брокера
  return []
}

// «Binance: начисление 0.011 USDT» → «0.011 USDT»: слово вида уже в бейдже
export const cleanTitle = (t, title) => (t.kind ? title.replace(/^Binance:\s*\S+\s+/, '') : title)

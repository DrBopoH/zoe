import { valueAt, lastPoint } from './macro.js'

// Крипто-контур «банк → P2P USDT → торговля → вывод»: производные ряды P2P и стоимость переводов. Чистые функции, без интерфейса.

// Спред покупки и продажи USDT со знаком (% от середины): покупка дороже продажи — плюс (обычный рынок), продажа дороже покупки — минус (перевёрнутый) и переплата к официальному курсу НБУ: по датам, где есть и покупка, и продажа.
// ask — сколько гривен стоит купить USDT, bid — сколько дают за USDT при продаже.
export function p2pDerived(series) {
  const ask = series?.p2p_ask || []
  const bid = series?.p2p_bid || []
  const nbu = series?.usd_uah
  const bidAt = new Map(bid.map((p) => [p.d, p.v]))
  const spread = []
  const premAsk = []
  const premBid = []
  for (const a of ask) {
    const b = bidAt.get(a.d)
    const r = valueAt(nbu, a.d)
    if (b > 0) spread.push({ d: a.d, v: round(((a.v - b) / ((a.v + b) / 2)) * 100) })
    if (r > 0) premAsk.push({ d: a.d, v: round((a.v / r - 1) * 100) })
    if (r > 0 && b > 0) premBid.push({ d: a.d, v: round((b / r - 1) * 100) })
  }
  return { p2p_spread: spread, p2p_prem_ask: premAsk, p2p_prem_bid: premBid }
}
const round = (v) => Math.round(v * 100) / 100

// Строки «сколько стоит перевести USDT»: последняя комиссия сети в долларах и в гривнах по курсу НБУ
export const FEE_NETWORKS = [
  { id: 'fee_tron_usd', label: 'TRON (USDT-TRC20)', note: 'энергия 65 000 и трафик, сжигание TRX' },
  { id: 'fee_eth_usd', label: 'Ethereum (USDT-ERC20)', note: '65 000 газа по текущей цене' },
  { id: 'fee_btc_usd', label: 'Bitcoin (перевод BTC)', note: '140 vB по ставке «полчаса»' }
]

export function feeRows(series) {
  const rate = lastPoint(series?.usd_uah)?.v ?? null
  return FEE_NETWORKS.map((n) => {
    const last = lastPoint(series?.[n.id])
    return { ...n, usd: last ? last.v : null, uah: last && rate ? Math.round(last.v * rate * 100) / 100 : null, date: last ? last.d : null }
  })
}

// Фандинг словами: положительный — лонги платят шортам (рынок перегрет вверх), отрицательный — наоборот
export function fundingLevel(v) {
  if (v === null || v === undefined || !isFinite(v)) return null
  return v > 30 ? 'сильно перегрет вверх' : v > 8 ? 'лонги платят' : v >= -3 ? 'нейтрально' : 'шорты платят'
}

import { WALLET } from './txTags.js'
import { fixedText, numText } from './numfmt.js'

// Состав счёта на карточке (account.portfolio с бэкенда).

// Цвет названия кошелька — тот же ряд, что у бейджей в истории
const WALLET_TEXT = {
  Spot: 'text-slate-200',
  Funding: 'text-cyan-300',
  Earn: 'text-emerald-300',
  Strategy: 'text-fuchsia-300',
  'USD-M Futures': 'text-orange-300'
}
export const walletName = (w) => (WALLET[w] ? WALLET[w][0] : String(w).toUpperCase())
export const walletText = (w) => WALLET_TEXT[w] || 'text-slate-300'

const STABLE = new Set(['USDT', 'USDC', 'FDUSD', 'BUSD', 'TUSD', 'USDP', 'DAI', 'UAH'])

// Количество монеты: у стейблкоинов копейки, у остальных до 8 знаков без хвостовых нулей
export const fmtQty = (asset, qty) => {
  if (STABLE.has(asset)) return fixedText(Math.round(qty * 100) / 100, 2)
  return numText(Number(qty.toFixed(8)), 8) // пыль меньше 1e-8 — «0», как и раньше
}

// Кошельки к показу: пустые скрыты (бэкенд уже не присылает их, здесь — на случай старых данных)
export const walletsOf = (card) => {
  const p = card && card.portfolio
  if (!p || p.kind !== 'wallets' || !Array.isArray(p.wallets)) return []
  return p.wallets.filter(w => w.items && w.items.length)
}
export const bondsOf = (card) => {
  const p = card && card.portfolio
  return p && p.kind === 'bonds' ? p : null
}

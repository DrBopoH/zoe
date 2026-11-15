import test from 'node:test'
import assert from 'node:assert/strict'
import { p2pDerived, feeRows, fundingLevel, FEE_NETWORKS } from '../src/utils/chain.js'
import { SERIES } from '../src/utils/macro.js'
import { MARKET_SECTIONS, feedOf } from '../src/utils/marketNav.js'

test('спред и переплата к курсу НБУ по датам, где есть обе стороны', () => {
  const series = {
    p2p_ask: [{ d: '2026-11-10', v: 47.2 }, { d: '2026-11-16', v: 47.5 }],
    p2p_bid: [{ d: '2026-11-16', v: 46.5 }],
    usd_uah: [{ d: '2026-11-01', v: 42 }]
  }
  const r = p2pDerived(series)
  assert.deepEqual(r.p2p_spread, [{ d: '2026-11-16', v: 2.13 }])
  assert.equal(r.p2p_prem_ask.length, 2)
  assert.equal(r.p2p_prem_ask[1].v, 13.1)
  assert.deepEqual(r.p2p_prem_bid, [{ d: '2026-11-16', v: 10.71 }])
})

test('нет рядов — пустые производные', () => {
  assert.deepEqual(p2pDerived({}), { p2p_spread: [], p2p_prem_ask: [], p2p_prem_bid: [] })
  assert.deepEqual(p2pDerived({ p2p_ask: [{ d: '2026-11-16', v: 47 }] }).p2p_prem_ask, [])
})

test('комиссии сетей в долларах и гривнах; нет данных — null', () => {
  const rows = feeRows({ fee_tron_usd: [{ d: '2026-11-15', v: 2 }, { d: '2026-11-16', v: 2.05 }], usd_uah: [{ d: '2026-11-16', v: 42 }] })
  assert.equal(rows.length, FEE_NETWORKS.length)
  assert.equal(rows[0].usd, 2.05)
  assert.equal(rows[0].uah, 86.1)
  assert.equal(rows[1].usd, null)
  assert.equal(rows[1].uah, null)
})

test('фандинг словами', () => {
  assert.equal(fundingLevel(40), 'сильно перегрет вверх')
  assert.equal(fundingLevel(11), 'лонги платят')
  assert.equal(fundingLevel(0), 'нейтрально')
  assert.equal(fundingLevel(-10), 'шорты платят')
  assert.equal(fundingLevel(null), null)
})

test('подписи рядов контура, раздел в навигации читает источник chain', () => {
  const ids = ['p2p_ask', 'p2p_bid', 'p2p_spread', 'p2p_prem_ask', 'p2p_prem_bid', 'fund_btc', 'fund_eth', 'oi_btc', 'oi_eth', 'stable_usdt', 'stable_usdc', 'fee_btc_sat', 'fee_btc_usd', 'fee_tron_usd', 'fee_eth_usd', 'dom_btc', 'mcap_total']
  for (const id of ids) assert.ok(SERIES[id]?.label && SERIES[id]?.color, id)
  assert.ok(MARKET_SECTIONS.includes('chain'))
  assert.equal(feedOf('chain'), 'chain')
})

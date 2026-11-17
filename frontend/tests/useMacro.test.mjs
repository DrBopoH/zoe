import test from 'node:test'
import assert from 'node:assert/strict'
import { isShallow, isReactive } from 'vue'
import { feedSeries, countryList, snapshotIds, loadTicks, resetCatalog, useMacro } from '../src/composables/useMacro.js'
import { useCountries } from '../src/composables/useCountries.js'

test('каталог спрашивается один раз на все источники', async () => {
  resetCatalog()
  let calls = 0
  const get = async () => { calls++; return { feeds: { fx: ['usd_uah'], us: ['fed_funds'] } } }
  assert.deepEqual(await feedSeries('fx', get), ['usd_uah'])
  assert.deepEqual(await feedSeries('us', get), ['fed_funds'])
  assert.deepEqual(await feedSeries('нет', get), [])
  assert.equal(calls, 1)
})

test('сбой каталога не залипает: следующий вызов пробует снова', async () => {
  resetCatalog()
  await assert.rejects(feedSeries('fx', async () => { throw new Error('нет связи') }))
  assert.deepEqual(await feedSeries('fx', async () => ({ feeds: { fx: ['usd_uah'] } })), ['usd_uah'])
})

test('реестр стран приходит из того же каталога: один запрос на источники и страны', async () => {
  resetCatalog()
  let calls = 0
  const get = async () => { calls++; return { feeds: { ez: ['ez_policy'] }, countries: [{ cc: 'ez', name: 'Еврозона', metrics: { policy: 'ez_policy' } }] } }
  assert.deepEqual(await feedSeries('ez', get), ['ez_policy'])
  assert.equal((await countryList(get))[0].cc, 'ez')
  assert.equal(calls, 1)
  resetCatalog()
  assert.deepEqual(await countryList(async () => ({ feeds: {} })), [])
})

test('часовые точки запрашиваются только для рядов-снимков из каталога; сбой одного ряда не ломает остальные', async () => {
  resetCatalog()
  const asked = []
  const get = async (path) => {
    if (path === '/macro/catalog') return { feeds: { chain: ['p2p_ask', 'p2p_bid', 'fund_btc'] }, snapshots: ['p2p_ask', 'p2p_bid', 'dom_btc'] }
    asked.push(path)
    if (path.endsWith('p2p_bid')) throw new Error('нет')
    return [{ t: 1, v: 2 }]
  }
  assert.deepEqual(await snapshotIds(get), ['p2p_ask', 'p2p_bid', 'dom_btc'])
  const r = await loadTicks(['p2p_ask', 'p2p_bid', 'fund_btc'], get)
  assert.deepEqual(asked.sort(), ['/macro/ticks?series=p2p_ask', '/macro/ticks?series=p2p_bid'])
  assert.deepEqual(r, { p2p_ask: [{ t: 1, v: 2 }], p2p_bid: [] })
  resetCatalog()
  assert.deepEqual(await snapshotIds(async () => ({ feeds: {} })), [])
})

test('ряды не оборачиваются глубокой реактивностью: чтение сотен тысяч точек через Proxy тормозило экран «Курсы»', () => {
  for (const { series } of [useMacro('fx'), useCountries()]) {
    assert.equal(isShallow(series), true)
    series.value = { usd_uah: [{ d: '2026-01-01', v: 40 }] }
    assert.equal(isReactive(series.value.usd_uah), false)
  }
})

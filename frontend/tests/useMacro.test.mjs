import test from 'node:test'
import assert from 'node:assert/strict'
import { feedSeries, countryList, resetCatalog } from '../src/composables/useMacro.js'

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

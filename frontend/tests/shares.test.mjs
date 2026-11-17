import test from 'node:test'
import assert from 'node:assert/strict'
import { stockSeries, stockRatios, capCheck, usdText, ratioText } from '../src/utils/shares.js'
import { useStocks, STOCK_LIST_LIMIT } from '../src/composables/useStocks.js'
import { MARKET_TREE } from '../src/utils/marketNav.js'
import { SERIES, unitText, axisText } from '../src/utils/macro.js'

const detail = {
  prices: [{ d: '2026-09-01', c: 250, a: 249 }, { d: '2026-10-01', c: 0, a: 0 }, { d: '2026-11-01', c: 270, a: 269 }],
  cap: [{ d: '2026-09-01', v: 3.7e12 }, { d: '2026-11-01', v: 4.0e12 }],
  fundamentals: {
    revenue: [{ d: '2025-09-27', v: 391e9 }],
    net_income: [{ d: '2025-09-27', v: 94e9 }],
    equity: [{ d: '2025-09-27', v: 74e9 }],
    assets: [{ d: '2025-09-27', v: 359e9 }]
  }
}

test('ряды для графиков: цена в $, остальное в млрд $, нулевая цена отброшена', () => {
  const s = stockSeries(detail)
  assert.deepEqual(s.sh_price.map((p) => p.d), ['2026-09-01', '2026-11-01'])
  assert.equal(s.sh_cap[1].v, 4000)
  assert.equal(s.sh_revenue[0].v, 391)
  assert.equal(s.sh_income[0].v, 94)
  assert.deepEqual(stockSeries(null).sh_cap, [])
  for (const id of Object.keys(s)) assert.ok(SERIES[id], `у ряда ${id} нет описания в SERIES`)
})

test('мультипликаторы: по свежей капитализации и последним отчётам; убыток и отрицательный капитал не считаются', () => {
  const r = stockRatios(detail, '2026-11-17')
  assert.equal(r.cap, 4.0e12)
  assert.ok(Math.abs(r.ps - 4.0e12 / 391e9) < 1e-9)
  assert.ok(Math.abs(r.pe - 4.0e12 / 94e9) < 1e-9)
  assert.ok(Math.abs(r.pb - 4.0e12 / 74e9) < 1e-9)
  const loss = { ...detail, fundamentals: { ...detail.fundamentals, net_income: [{ d: '2025-12-31', v: -5e9 }], equity: [{ d: '2025-09-27', v: -1e9 }] } }
  const l = stockRatios(loss, '2026-11-17')
  assert.equal(l.pe, null)
  assert.equal(l.pb, null)
  assert.ok(l.ps > 0)
  // отчёт старше полутора лет — не применяется
  assert.equal(stockRatios(detail, '2027-12-01').ps, null)
  assert.equal(stockRatios({ cap: [] }, '2026-11-17'), null)
})

test('сверка с Nasdaq: расхождение больше 25 % — предупреждение, старая точка — не сверяем', () => {
  const ok = capCheck(4.1e12, detail.cap, '2026-11-17')
  assert.ok(ok.ok && Math.abs(ok.diff + 2.4) < 0.1)
  const bad = capCheck(2.0e12, [{ d: '2026-11-01', v: 1.0e12 }], '2026-11-17') // как GOOGL: в отчёте один класс
  assert.equal(bad.ok, false)
  assert.equal(Math.round(bad.diff), -50)
  assert.equal(capCheck(4e12, [{ d: '2026-01-01', v: 4e12 }], '2026-11-17'), null)
  assert.equal(capCheck(0, detail.cap, '2026-11-17'), null)
  assert.equal(capCheck(4e12, [], '2026-11-17'), null)
})

test('подписи: капитализация словами, нет значения — тире', () => {
  assert.equal(usdText(4.0e12), '4 трлн $')
  assert.equal(usdText(0), '—')
  assert.equal(usdText(-5e9), '−5 млрд $')
  assert.equal(usdText(391e9), '391 млрд $')
  assert.equal(ratioText(null), '—')
  assert.equal(ratioText(42.5), '42,5')
})

test('раздел «Акции компаний» стоит в макро и обновляет три источника сразу', () => {
  const macro = MARKET_TREE.find((g) => g.key === 'macro').children
  const s = macro.find((c) => c.key === 'shares')
  assert.equal(s.feed, 'companies,stockprices,stockfacts')
})

test('поиск и открытие компании: берётся только последний ответ каждого вида', async () => {
  const resolvers = {}
  const get = (path) => new Promise((res) => { resolvers[path] = res })
  const st = useStocks(get)
  const a = st.search('app'), b = st.search('apple')
  resolvers[`/market/companies?q=apple&limit=${STOCK_LIST_LIMIT}`]([{ ticker: 'AAPL' }])
  resolvers[`/market/companies?q=app&limit=${STOCK_LIST_LIMIT}`]([{ ticker: 'APP' }])
  await Promise.all([a, b])
  assert.deepEqual(st.list.value.map((c) => c.ticker), ['AAPL'], 'старый ответ не затирает новый')
  const o1 = st.open('AAPL'), o2 = st.open('TSLA')
  resolvers['/market/stock?ticker=TSLA']({ company: { ticker: 'TSLA' } })
  resolvers['/market/stock?ticker=AAPL']({ company: { ticker: 'AAPL' } })
  await Promise.all([o1, o2])
  assert.equal(st.detail.value.company.ticker, 'TSLA')
  assert.equal(st.loading.value, false)
})

test('сбой открытия компании: ошибка с текстом сервера, карточки нет', async () => {
  const st = useStocks(async () => { throw new Error('компании ZZZ нет в списке') })
  await st.open('ZZZ')
  assert.equal(st.detail.value, null)
  assert.match(st.error.value, /нет в списке/)
  await st.open('')
  assert.equal(st.detail.value, null)
})

test('единица «млрд»: тысячи сокращаются до трлн, чтобы подпись оси влезала', () => {
  assert.equal(unitText(5402, 'млрд'), '5,4 трлн')
  assert.equal(unitText(390, 'млрд'), '390 млрд')
  assert.equal(unitText(4.5, 'млрд'), '4,5 млрд')
  assert.equal(axisText(2000, 'млрд'), '2 трлн')
})

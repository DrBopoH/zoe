import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { priceCandidates, priceFromLine, mergeEntries, simpleToEffective, parseIcuScreen } from '../src/utils/icuOcr.js'

const fx = (n) => readFileSync(new URL(`./fixtures/icu-${n}.txt`, import.meta.url), 'utf8')
const today = '2026-11-12'
// реестр НБУ (срез по бумагам со скриншотов: ISIN, погашение, купон)
const R = [
  ['239081', '2030-02-20', 16.44], ['238992', '2029-04-25', 16.16], ['239131', '2029-03-21', 16.49], ['238968', '2029-03-07', 17.48],
  ['239107', '2029-02-07', 16.07], ['235782', '2028-11-29', 17.8], ['237556', '2028-11-15', 17.8], ['236475', '2028-09-27', 17.77],
  ['239008', '2028-06-14', 15.78], ['239040', '2028-04-26', 15.85], ['235196', '2028-04-12', 17.8], ['237804', '2028-03-22', 17.5],
  ['231625', '2028-02-09', 16.8], ['239115', '2027-09-15', 15.18], ['239016', '2027-07-21', 15.15], ['230270', '2027-07-07', 18.5],
  ['234223', '2027-06-09', 16.2], ['207518', '2027-05-26', 9.79], ['236228', '2027-03-10', 17.1], ['234413', '2027-01-13', 16.61],
  ['237416', '2026-11-18', 16.35]
].map(([n, maturity, coupon_rate]) => ({ id: `UA4000${n}`, maturity, coupon_rate, nominal: 1000 }))

test('варианты цены из строки «Кількість»: количество не мешает, лишние пробелы склеиваются', () => {
  assert.ok(priceCandidates('Kinbkictb: 9 566 1 038,68 =').includes(1038.68))
  assert.ok(priceCandidates('Kinbkictb: 31 161 1067,95¢e').includes(1067.95))
  assert.ok(priceCandidates('Kinbkictb: 11 146 1 043,662').includes(1043.66)) // «2» вместо знака ₴
  assert.ok(priceCandidates('Kinbkictb: 130 1 079,75 e').includes(1079.75))
  assert.ok(priceCandidates('Kinbkictb: 101 1 077,1 9 e').includes(1077.19)) // пробел внутри копеек
  assert.ok(priceCandidates('Kinbkictb: 21 427 1 1 1 7,72 e').includes(1117.72)) // пробелы внутри тысяч
  assert.equal(priceFromLine('Kinbkictb: 9 566 1 038,68 ='), 1038.68)
  assert.deepEqual(priceCandidates('Кількість: 3'), [])
  assert.deepEqual(priceCandidates(''), [])
})

test('SIM для коротких бумаг переводится в эффективную годовую', () => {
  assert.ok(Math.abs(simpleToEffective(14.1, 0.32) - 14.79) < 0.01)
  assert.equal(simpleToEffective(15, 2), 15) // длиннее года простая не применяется
})

test('скриншоты ICU: ISIN чинятся по реестру, цены читаются, битые цены отбрасываются и остаётся YTM', () => {
  const all = ['fc39caf0', '73cefdb2', '0a735a55'].flatMap((n) => parseIcuScreen(fx(n), R, today))
  const by = Object.fromEntries(all.map((e) => [e.isin, e]))
  assert.equal(all.length, 21) // по 7 карточек на каждом из трёх экранов
  assert.equal(by.UA4000239081.price, 1038.68)
  assert.equal(by.UA4000239081.ytm, 17)
  assert.deepEqual(by.UA4000239081.flags, []) // «ya4o00239081»: буква «o» вместо нуля читается как цифра
  assert.equal(by.UA4000239131.price, 1033.38)
  assert.equal(by.UA4000238992.price, 1013.46) // «3 101 3,46» вместо 1 013,46: склейка «1013» сходится с YTM 16,5
  assert.equal(by.UA4000238992.ytm, 16.5) // доходность осталась
  assert.equal(by.UA4000235782.price, 1117.72) // «1 1 1 7,72»: цена восстановлена склейкой и сверена с YTM 16,2
  assert.equal(by.UA4000239115.price, null) // цену срезало панелью
  assert.equal(by.UA4000239115.ytm, 15)
  assert.equal(by.UA4000236228.kind, 'SIM')
  assert.ok(by.UA4000236228.ytm > 14.1 && by.UA4000236228.ytm < 15.5) // 14,1 % простых за 0,3 года ≈ 14,8 % годовых
  assert.equal(by.UA4000237416.price, 1079.75)
  assert.equal(by.UA4000234223.price, 1077.19) // «1 077,1 9»
})

test('те же бумаги с перекрывающихся скриншотов не дублируются внутри одного кадра и совпадают между кадрами', () => {
  const a = parseIcuScreen(fx('fc39caf0'), R, today).map((e) => e.isin)
  const b = parseIcuScreen(fx('5417abd6'), R, today)
  assert.equal(new Set(a).size, a.length)
  const price = Object.fromEntries(b.map((e) => [e.isin, e.price]))
  assert.equal(price.UA4000239081, 1038.68)
  assert.equal(price.UA4000239131, 1033.38) // на этом кадре ISIN прочитан как ...230131: найден по дате погашения
  assert.ok(b.find((e) => e.isin === 'UA4000239131').flags.includes('isin-fixed'))
  assert.ok(b.find((e) => e.isin === 'UA4000238992').flags.includes('isin-fixed')) // ...238092
})

test('мусор и пустой текст не падают; бумаги вне реестра помечаются', () => {
  assert.deepEqual(parseIcuScreen('', R, today), [])
  assert.deepEqual(parseIcuScreen('привет\n123', R, today), [])
  const x = parseIcuScreen('UA4000999999 YTM 15%\nПогашення 01.01.2031\nКількість: 5 1 000,00 e', R, today)
  assert.equal(x.length, 1)
  assert.ok(x[0].flags.includes('not-in-registry') || x[0].flags.includes('isin-fixed'))
})

test('слияние проходов и скриншотов: запись с ценой побеждает запись без неё', () => {
  const a = [{ isin: 'UA4000A', price: null, flags: ['price-bad'] }, { isin: 'UA4000B', price: 1000, flags: [] }]
  const b = [{ isin: 'UA4000A', price: 1010, flags: [] }, { isin: 'UA4000B', price: null, flags: ['price-none'] }]
  const m = mergeEntries([a, b])
  assert.equal(m.find((e) => e.isin === 'UA4000A').price, 1010)
  assert.equal(m.find((e) => e.isin === 'UA4000B').price, 1000)
  assert.equal(m.length, 2)
})

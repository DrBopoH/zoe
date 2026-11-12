import test from 'node:test'
import assert from 'node:assert/strict'
import { BANKS, bankKeyByMfo, bankKeyByCard, BANK_ORDER, bankLabel } from '../src/utils/banks.js'
import { detectBankKey, pseudoCounterName } from '../src/utils/personLayout.js'
import { bankMark } from '../src/utils/ibanMask.js'

test('реестр: МФО и BIN однозначны (код банка не принадлежит двум банкам)', () => {
  const mfos = Object.values(BANKS).flatMap((b) => b.mfo)
  assert.equal(new Set(mfos).size, mfos.length)
  const bins = Object.values(BANKS).flatMap((b) => b.bin)
  assert.equal(new Set(bins).size, bins.length)
})

test('банк по МФО и по карте', () => {
  assert.equal(bankKeyByMfo('322001'), 'mono')
  assert.equal(bankKeyByMfo('380805'), 'raif')
  assert.equal(bankKeyByMfo('000000'), '')
  assert.equal(bankKeyByCard('5168123412341234'), 'privat')
  assert.equal(bankKeyByCard('537541******1111'), 'mono')
  assert.equal(bankKeyByCard('9999'), '')
})

test('порядок групп на карточке Лица', () => {
  assert.deepEqual(BANK_ORDER, ['mono', 'raif', 'privat', 'novapay', 'other'])
})

test('detectBankKey: IBAN, профиль Binance, карты, тип счёта', () => {
  assert.equal(detectBankKey('UA213220010000026007233566001', null, []), 'mono')
  assert.equal(detectBankKey('UA533052990000000000000006003', null, []), 'privat')
  assert.equal(detectBankKey('UA12935871000000000000000001', null, []), 'novapay')
  assert.equal(detectBankKey('UA673809460000000000000005004', null, []), 'icu')
  assert.equal(detectBankKey('BN12345678', null, []), 'binance')
  assert.equal(detectBankKey('', null, ['414949******0172']), 'privat')
  assert.equal(detectBankKey('', { type: 'black' }, []), 'mono')
  assert.equal(detectBankKey('', { bank_key: 'icu' }, []), 'icu')
  assert.equal(detectBankKey('', null, []), 'other')
})

test('имя контрагента по карте: карта · банк', () => {
  assert.equal(pseudoCounterName({ counter_iban: '487407****44' }), '487407****44 · Monobank')
  assert.equal(bankLabel('novapay'), 'NovaPay')
})

test('значок банка по МФО берётся из реестра', () => {
  assert.equal(bankMark('935871').letter, 'N')
  assert.equal(bankMark('380805').name, 'Райффайзен') // второй МФО Райффайзена раньше значка не имел
  assert.equal(bankMark('123456').small, true)
})

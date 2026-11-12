import test from 'node:test'
import assert from 'node:assert/strict'
import { marketNotice } from '../src/utils/marketNotice.js'

const last = (at, feeds) => ({ finished_at: at, feeds })
const ok = { id: 'nbu', title: 'ОВДП · НБУ', state: 'done', fetched: 184 }
const bad = { id: 'ust', title: 'Облигации США', state: 'error', fetched: 0, error: 'HTTP 502' }

test('итог до перезагрузки страницы не показывается, а запоминается', () => {
  assert.deepEqual(marketNotice(null, last(100, [ok])), { seenAt: 100, toasts: [] })
})

test('нет итога или старый итог — тостов нет', () => {
  assert.deepEqual(marketNotice(100, null), { seenAt: 100, toasts: [] })
  assert.deepEqual(marketNotice(100, last(100, [ok])), { seenAt: 100, toasts: [] })
  assert.deepEqual(marketNotice(0, { finished_at: 5 }), { seenAt: 0, toasts: [] }) // сервер без поля feeds
})

test('новый итог: по тосту на источник, упавший красный и не исчезает сам, успех — по таймеру', () => {
  const r = marketNotice(100, last(200, [ok, bad]))
  assert.equal(r.seenAt, 200)
  assert.equal(r.toasts.length, 2)
  assert.deepEqual(r.toasts[0], { kind: 'ok', title: 'ОВДП · НБУ', lines: ['Бумаг в кеше: 184'], ttl: 6000 })
  assert.equal(r.toasts[1].kind, 'err')
  assert.equal(r.toasts[1].ttl, undefined)
  assert.deepEqual(r.toasts[1].lines, ['HTTP 502'])
})

test('упавший источник не прячет успех другого, повторный опрос того же итога молчит', () => {
  const l = last(300, [bad, ok])
  const a = marketNotice(200, l)
  assert.equal(a.toasts.length, 2)
  assert.equal(marketNotice(a.seenAt, l).toasts.length, 0)
})

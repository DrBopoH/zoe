import test from 'node:test'
import assert from 'node:assert/strict'

// localStorage в Node нет — storage.js должен переживать это: по умолчанию автозапись включена
const { syncPath, autoSave, setAutoSave } = await import('../src/composables/useSync.js')

test('путь запуска: с галочкой просим автозапись', () => {
  assert.equal(syncPath(true), '/sync/full?save=1')
  assert.equal(syncPath(false), '/sync/full')
})

test('по умолчанию автозапись включена, переключатель меняет значение', () => {
  assert.equal(autoSave.value, true)
  setAutoSave(false)
  assert.equal(autoSave.value, false)
  setAutoSave(true)
  assert.equal(autoSave.value, true)
})

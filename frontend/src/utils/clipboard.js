// Единственная точка чтения буфера обмена.
// ВАЖНО: вызывать readClipboardText() прямо в обработчике клика (первым await), без
// preventDefault/stopPropagation на самом событии и без промежуточных слоёв —
// так браузер считает вызов жестом пользователя и не показывает «Вставить».
export async function readClipboardText() {
  if (!navigator.clipboard?.readText) throw new Error('Clipboard API недоступен')
  const text = await navigator.clipboard.readText()
  return String(text || '').replace(/[\x00-\x1F\x7F-\x9F]/g, '').trim()
}

export async function writeClipboardText(text) {
  try {
    await navigator.clipboard?.writeText(text)
  } catch (err) {
    console.error('Clipboard write error:', err)
  }
}

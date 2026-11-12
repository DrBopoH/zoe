// Распознавание скриншота экрана брокера в браузере (tesseract.js, всё локально: картинка никуда не уходит).
// Ядро, воркер и модель английского лежат в node_modules и подтягиваются только при первом распознавании.
// Здесь только запуск; разбор текста — в icuOcr.js.
import workerUrl from 'tesseract.js/dist/worker.min.js?url'
import coreUrl from 'tesseract.js-core/tesseract-core-simd-lstm.wasm.js?url'
import dataUrl from '@tesseract.js-data/eng/4.0.0_best_int/eng.traineddata.gz?url'

const abs = (u) => new URL(u, window.location.href).href

let workerP = null
const getWorker = async () => {
  if (!workerP) {
    workerP = import('tesseract.js').then(({ createWorker, PSM }) =>
      createWorker('eng', 1, {
        workerPath: abs(workerUrl),
        corePath: abs(coreUrl),
        langPath: abs(dataUrl.slice(0, dataUrl.lastIndexOf('/'))),
        gzip: true,
        cacheMethod: 'none'
      }).then(async (w) => { await w.setParameters({ tessedit_pageseg_mode: PSM.SINGLE_BLOCK }); return w })
    ).catch((e) => { workerP = null; throw e })
  }
  return workerP
}

// Светлый текст на тёмном фоне → тёмный на белом, без шапки и нижней панели приложения (у высокого кадра телефона).
// Несколько проходов с разным порогом и масштабом: цифры, которые не читаются в одном, читаются в другом.
export const PASSES = [{ thr: 150, scale: 1 }, { thr: 175, scale: 1.5 }, { thr: 125, scale: 1.5 }]

export async function prepareImage(blob, { thr = 150, scale = 1 } = {}) {
  const bmp = await createImageBitmap(blob)
  const tall = bmp.height / bmp.width > 1.8
  const y0 = tall ? Math.round(bmp.height * 0.17) : 0
  const y1 = tall ? Math.round(bmp.height * 0.89) : bmp.height
  const c = document.createElement('canvas')
  c.width = Math.round(bmp.width * scale); c.height = Math.round((y1 - y0) * scale)
  const ctx = c.getContext('2d', { willReadFrequently: true })
  ctx.imageSmoothingQuality = 'high'
  ctx.drawImage(bmp, 0, y0, bmp.width, y1 - y0, 0, 0, c.width, c.height)
  const img = ctx.getImageData(0, 0, c.width, c.height)
  const d = img.data
  for (let i = 0; i < d.length; i += 4) {
    const g = 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2]
    const v = g > thr ? 0 : 255
    d[i] = d[i + 1] = d[i + 2] = v; d[i + 3] = 255
  }
  ctx.putImageData(img, 0, 0)
  return c
}

// Тексты по всем проходам одного скриншота (разбор и слияние — в icuOcr.js)
export async function recognizeScreen(blob, onProgress) {
  const worker = await getWorker()
  const texts = []
  for (let i = 0; i < PASSES.length; i++) {
    const canvas = await prepareImage(blob, PASSES[i])
    const { data } = await worker.recognize(canvas)
    texts.push(data.text || '')
    if (onProgress) onProgress((i + 1) / PASSES.length)
  }
  return texts
}

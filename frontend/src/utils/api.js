// Адрес бэкенда: VITE_API_BASE в .env фронта, иначе локальный бэкенд.
export const API_BASE = import.meta.env?.VITE_API_BASE || 'http://localhost:8080/api'

async function request(method, path, opts) {
  const res = await fetch(`${API_BASE}${path}`, { method, ...opts })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    const err = new Error(text || `${method} ${path}: ${res.status}`)
    err.status = res.status // 409 — «уже идёт»: это не поломка, интерфейс показывает её иначе
    throw err
  }
  return res
}

// Универсальный запрос: бросает Error с текстом бэкенда, если статус не 2xx. body (если есть) уходит как JSON.
export function apiSend(method, path, body) {
  if (body === undefined) return request(method, path)
  return request(method, path, { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
}

// Загрузка файла (FormData): заголовок Content-Type браузер ставит сам, с границей multipart.
export const apiUpload = (path, formData, method = 'POST') => request(method, path, { body: formData })

export async function apiGetJson(path) {
  return (await apiSend('GET', path)).json()
}

// Запрос с телом/без и разбором JSON-ответа.
export async function apiJson(method, path, body) {
  return (await apiSend(method, path, body)).json()
}

export const q = encodeURIComponent

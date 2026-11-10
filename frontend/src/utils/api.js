export const API_BASE = 'http://localhost:8080/api'

// Универсальный запрос: бросает Error с текстом бэкенда, если статус не 2xx.
export async function apiSend(method, path, body) {
  const opts = { method }
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' }
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(`${API_BASE}${path}`, opts)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `${method} ${path}: ${res.status}`)
  }
  return res
}

export async function apiGetJson(path) {
  const res = await apiSend('GET', path)
  return res.json()
}

export const q = encodeURIComponent
